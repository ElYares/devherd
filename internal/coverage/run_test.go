package coverage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// probeRunner responde la sonda del contenedor con un estado dado, y registra lo
// que se le pidio. Asi las pruebas cubren la decision entera sin daemon de Docker.
type probeRunner struct {
	workingDir string
	uid        string
	pcov       string
	memory     string
	failProbe  error
	calls      [][]string
}

func (r *probeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.failProbe != nil {
		return "", r.failProbe
	}

	return strings.Join([]string{r.workingDir, r.uid, r.pcov, r.memory}, "\n"), nil
}

// laravelRunner reproduce tl-mas-server: root, sin driver, 128M.
func laravelRunner() *probeRunner {
	return &probeRunner{workingDir: "/app", uid: "0", pcov: "no", memory: "128M"}
}

func laravelOptions(runner *probeRunner) RunOptions {
	return RunOptions{
		Stack:       "laravel",
		ProjectRoot: "/proyectos/demo",
		ComposeArgs: []string{"docker", "compose", "-f", "docker-compose.yml"},
		Runner:      runner,
	}
}

func stepByTitle(t *testing.T, plan RunPlan, title string) Step {
	t.Helper()

	for _, step := range plan.Steps {
		if step.Title == title {
			return step
		}
	}

	t.Fatalf("step %q not found in plan: %#v", title, plan.Steps)

	return Step{}
}

func TestRunPlanInstallsTheDriverWhenMissing(t *testing.T) {
	plan, err := BuildRunPlan(context.Background(), laravelOptions(laravelRunner()))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	driver := stepByTitle(t, plan, "coverage driver")
	if driver.Skipped {
		t.Fatal("the driver step must not be skipped when pcov is missing")
	}

	command := strings.Join(driver.Command, " ")
	if !strings.Contains(command, "pecl install pcov") {
		t.Fatalf("expected the install command, got %q", command)
	}
	// Idempotente: sin el guard, una segunda corrida reinstalaria.
	if !strings.Contains(command, "grep -qix pcov ||") {
		t.Fatalf("the install must be guarded so it is idempotent, got %q", command)
	}
}

// tl-mas-server ya corre como root: forzar -u root cambiaria el dueno de lo que
// escriba el paso.
func TestRunPlanDoesNotForceRootWhenTheContainerAlreadyIsRoot(t *testing.T) {
	plan, err := BuildRunPlan(context.Background(), laravelOptions(laravelRunner()))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	if command := strings.Join(stepByTitle(t, plan, "coverage driver").Command, " "); strings.Contains(command, "-u root") {
		t.Fatalf("expected no -u root for a root container, got %q", command)
	}
}

// aang-server corre como uid 1000 y ahi `pecl install` falla por permisos.
func TestRunPlanInstallsAsRootWhenTheContainerIsNot(t *testing.T) {
	runner := laravelRunner()
	runner.uid = "1000"

	plan, err := BuildRunPlan(context.Background(), laravelOptions(runner))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	driver := stepByTitle(t, plan, "coverage driver")
	if !strings.Contains(strings.Join(driver.Command, " "), "-u root") {
		t.Fatalf("expected -u root for a non-root container, got %q", strings.Join(driver.Command, " "))
	}
	if !strings.Contains(driver.Reason, "root") {
		t.Fatalf("the reason should explain why it escalates, got %q", driver.Reason)
	}

	// Las pruebas se corren con el usuario original, no como root.
	if command := strings.Join(stepByTitle(t, plan, "tests").Command, " "); strings.Contains(command, "-u root") {
		t.Fatalf("tests must run as the original user, got %q", command)
	}
}

func TestRunPlanSkipsTheDriverWhenPresent(t *testing.T) {
	runner := laravelRunner()
	runner.pcov = "yes"

	plan, err := BuildRunPlan(context.Background(), laravelOptions(runner))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	driver := stepByTitle(t, plan, "coverage driver")
	if !driver.Skipped || len(driver.Command) != 0 {
		t.Fatalf("expected the driver step skipped, got %#v", driver)
	}
}

// El limite se sube por ini y nunca con -d: `artisan test` lanza el runner en un
// proceso hijo que no hereda los flags del padre.
func TestRunPlanRaisesMemoryThroughAnIniFile(t *testing.T) {
	plan, err := BuildRunPlan(context.Background(), laravelOptions(laravelRunner()))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	memory := stepByTitle(t, plan, "memory limit")
	if memory.Skipped {
		t.Fatal("128M is below the floor; the step must not be skipped")
	}

	command := strings.Join(memory.Command, " ")
	if !strings.Contains(command, "conf.d") {
		t.Fatalf("expected the limit written to conf.d, got %q", command)
	}
	if strings.Contains(command, "-d memory_limit") {
		t.Fatalf("-d does not reach the child process; it must not be used: %q", command)
	}
	if !strings.Contains(memory.Reason, "128M") {
		t.Fatalf("the reason should name the current limit, got %q", memory.Reason)
	}
}

// aang-server ya venia en 512M y sus 8.368 sentencias se midieron sin tocar nada.
func TestRunPlanLeavesASufficientMemoryLimitAlone(t *testing.T) {
	runner := laravelRunner()
	runner.memory = "512M"

	plan, err := BuildRunPlan(context.Background(), laravelOptions(runner))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	memory := stepByTitle(t, plan, "memory limit")
	if !memory.Skipped || len(memory.Command) != 0 {
		t.Fatalf("expected 512M to be left alone, got %#v", memory)
	}
}

func TestRunPlanTreatsUnlimitedMemoryAsEnough(t *testing.T) {
	runner := laravelRunner()
	runner.memory = "-1"

	plan, err := BuildRunPlan(context.Background(), laravelOptions(runner))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	if memory := stepByTitle(t, plan, "memory limit"); !memory.Skipped {
		t.Fatalf("unlimited memory needs no step, got %#v", memory)
	}
}

// El comando por defecto es `artisan test`, no `vendor/bin/phpunit`: los dos
// proyectos reales usan Pest y ese camino revienta con un error de bootstrap.
func TestRunPlanDefaultsToArtisanTestForLaravel(t *testing.T) {
	plan, err := BuildRunPlan(context.Background(), laravelOptions(laravelRunner()))
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	tests := stepByTitle(t, plan, "tests")
	command := strings.Join(tests.Command, " ")
	if !strings.Contains(command, "php artisan test --coverage-clover=") {
		t.Fatalf("expected artisan test, got %q", command)
	}
	if strings.Contains(command, "vendor/bin/phpunit") {
		t.Fatalf("phpunit breaks on Pest projects; it must not be the default: %q", command)
	}
	if !strings.Contains(tests.Reason, ".devherd.yml") {
		t.Fatalf("the reason should say how to override it, got %q", tests.Reason)
	}
}

func TestRunPlanPrefersTheDeclaredTestCommand(t *testing.T) {
	opts := laravelOptions(laravelRunner())
	opts.TestCommand = "php vendor/bin/pest --coverage-clover=custom.xml"

	plan, err := BuildRunPlan(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	tests := stepByTitle(t, plan, "tests")
	if !strings.Contains(strings.Join(tests.Command, " "), "vendor/bin/pest") {
		t.Fatalf("expected the declared command, got %q", strings.Join(tests.Command, " "))
	}
	if !strings.Contains(tests.Reason, "declared") {
		t.Fatalf("the reason should say it came from the manifest, got %q", tests.Reason)
	}
}

// Go no corre en contenedor: ni driver, ni memoria, ni compose.
func TestRunPlanForGoNeedsNoContainer(t *testing.T) {
	plan, err := BuildRunPlan(context.Background(), RunOptions{
		Stack:       "go",
		ProjectRoot: "/proyectos/devherd",
	})
	if err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected a single step for go, got %#v", plan.Steps)
	}
	if plan.Service != "" {
		t.Fatalf("go needs no compose service, got %q", plan.Service)
	}
	if !strings.HasSuffix(plan.ReportPath, ".devherd.coverage.out") {
		t.Fatalf("unexpected report path %q", plan.ReportPath)
	}
	if command := strings.Join(plan.Steps[0].Command, " "); !strings.Contains(command, "-coverprofile=") {
		t.Fatalf("expected a coverprofile run, got %q", command)
	}
}

func TestStackNeedsContainer(t *testing.T) {
	if needs, err := StackNeedsContainer("laravel"); err != nil || !needs {
		t.Fatalf("laravel needs a container: %v %v", needs, err)
	}
	if needs, err := StackNeedsContainer("go"); err != nil || needs {
		t.Fatalf("go needs no container: %v %v", needs, err)
	}
	if _, err := StackNeedsContainer("cobol"); err == nil {
		t.Fatal("expected an error for an unsupported stack")
	}
}

func TestBuildRunPlanRejectsAnUnsupportedStack(t *testing.T) {
	_, err := BuildRunPlan(context.Background(), RunOptions{Stack: "rails", ProjectRoot: "/x"})
	if err == nil {
		t.Fatal("expected an error for an unsupported stack")
	}
	for _, want := range SupportedRunStacks() {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the error should list the supported stacks; %q missing from %q", want, err)
		}
	}
}

// Si el contenedor no responde, el mensaje tiene que decir que hacer, no volcar el
// error crudo de Docker.
func TestBuildRunPlanExplainsAStoppedContainer(t *testing.T) {
	runner := laravelRunner()
	runner.failProbe = errors.New("service \"app\" is not running")

	_, err := BuildRunPlan(context.Background(), laravelOptions(runner))
	if err == nil {
		t.Fatal("expected an error when the container does not answer")
	}
	if !strings.Contains(err.Error(), "devherd up") {
		t.Fatalf("the error should say how to fix it, got %q", err)
	}
}

func TestBuildRunPlanRequiresComposeArgsForContainerStacks(t *testing.T) {
	opts := laravelOptions(laravelRunner())
	opts.ComposeArgs = nil

	if _, err := BuildRunPlan(context.Background(), opts); err == nil {
		t.Fatal("expected an error without compose arguments")
	}
}

// La sonda es de solo lectura y en una sola llamada: cada exec cuesta cerca de un
// segundo.
func TestBuildRunPlanProbesOnceAndReadsOnly(t *testing.T) {
	runner := laravelRunner()
	if _, err := BuildRunPlan(context.Background(), laravelOptions(runner)); err != nil {
		t.Fatalf("BuildRunPlan returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected a single probe call, got %d: %#v", len(runner.calls), runner.calls)
	}
	probe := strings.Join(runner.calls[0], " ")
	for _, forbidden := range []string{"pecl install", "artisan test", "conf.d"} {
		if strings.Contains(probe, forbidden) {
			t.Fatalf("the probe must not change anything; found %q in %q", forbidden, probe)
		}
	}
}

func TestParseMemoryLimit(t *testing.T) {
	cases := []struct {
		value string
		want  int64
		ok    bool
	}{
		{"128M", 128 * 1024 * 1024, true},
		{"512M", 512 * 1024 * 1024, true},
		{"1G", 1024 * 1024 * 1024, true},
		{"262144K", 262144 * 1024, true},
		{"134217728", 134217728, true},
		{"-1", -1, true},
		{"", 0, false},
		{"mucha", 0, false},
	}

	for _, tc := range cases {
		got, ok := parseMemoryLimit(tc.value)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("parseMemoryLimit(%q) = %d, %v; want %d, %v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}
