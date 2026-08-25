package coverage

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devherd/devherd/internal/runner"
)

// minMemoryLimit es el piso por debajo del cual se sube la memoria de PHP. No es
// un numero inventado: con los 128M por defecto, PCOV mata la suite de
// tl-mas-server a mitad con "Allowed memory size exhausted"; con los 512M que ya
// traia aang-server, sus 8.368 sentencias se midieron sin tocar nada.
const minMemoryLimit = 512 * 1024 * 1024

// raisedMemoryLimit es a lo que se sube cuando hace falta.
const raisedMemoryLimit = "1G"

// memoryLimitFile va en conf.d y no en la linea de comandos a proposito:
// `artisan test` lanza el runner en un **proceso hijo**, que no hereda los `-d`
// del padre. Con `-d memory_limit=1G` la suite sigue muriendo a 128M.
const memoryLimitFile = "/usr/local/etc/php/conf.d/zz-devherd-coverage.ini"

// ManagedReportPrefix identifica los reportes que escribe DevHerd. Tienen que
// vivir dentro del proyecto porque el contenedor solo ve eso montado, asi que el
// prefijo es lo que los distingue de un archivo del usuario.
const ManagedReportPrefix = ".devherd.coverage"

// Step es una accion del plan. Se describe antes de ejecutarse para que --explain
// pueda imprimirla sin correr nada.
type Step struct {
	Title   string   `json:"title"`
	Reason  string   `json:"reason"`
	Command []string `json:"command,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	// Skipped marca lo que no hace falta hacer. Se conserva en el plan en vez de
	// omitirse: saber que algo ya estaba resuelto vale tanto como hacerlo.
	Skipped bool `json:"skipped"`
}

// RunPlan es todo lo que hay que hacer para producir un reporte, ya resuelto
// contra el estado real del contenedor.
type RunPlan struct {
	Stack      string `json:"stack"`
	Service    string `json:"service,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	// ReportPath es la ruta en el host, dentro del proyecto.
	ReportPath string `json:"report_path"`
	Steps      []Step `json:"steps"`
}

// RunOptions describe el proyecto sobre el que se va a trabajar.
type RunOptions struct {
	Stack       string
	ProjectRoot string
	// Service es el servicio compose donde corren las pruebas. Vacio usa el
	// predeterminado del stack.
	Service string
	// ComposeArgs es el prefijo ya resuelto del proyecto, tipicamente
	// ["docker", "compose", "-f", ...]. Vacio en stacks que corren en el host.
	ComposeArgs []string
	// TestCommand permite declarar el comando en `.devherd.yml`. Sin el se usa el
	// del stack, que no siempre acierta: tl-mas-server y aang-server usan Pest, y
	// `vendor/bin/phpunit` revienta con un error de bootstrap en los dos.
	TestCommand string
	Runner      runner.Runner
}

type stackProfile struct {
	service     string
	reportExt   string
	testCommand func(report string) string
	// needsContainer distingue los stacks que corren dentro de Docker de los que
	// corren en el host. Go no necesita preparacion de ningun tipo.
	needsContainer bool
}

func stackProfiles() map[string]stackProfile {
	return map[string]stackProfile{
		"laravel": {
			service:   "app",
			reportExt: ".xml",
			testCommand: func(report string) string {
				return "php artisan test --coverage-clover=" + report
			},
			needsContainer: true,
		},
		"go": {
			service:   "",
			reportExt: ".out",
			testCommand: func(report string) string {
				return "go test ./... -coverprofile=" + report
			},
			needsContainer: false,
		},
	}
}

// SupportedRunStacks lista los stacks que `--run` sabe preparar.
func SupportedRunStacks() []string {
	profiles := stackProfiles()
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	// Orden fijo para que los mensajes de error no cambien entre corridas.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	return names
}

// StackNeedsContainer dice si el stack corre dentro de Docker. Se consulta antes
// de resolver compose: un proyecto Go no tiene archivos compose y exigirselos lo
// dejaba fuera del comando.
func StackNeedsContainer(stack string) (bool, error) {
	profile, ok := stackProfiles()[strings.ToLower(strings.TrimSpace(stack))]
	if !ok {
		return false, fmt.Errorf(
			"coverage --run does not support stack %q yet; supported stacks: %s",
			stack, strings.Join(SupportedRunStacks(), ", "))
	}

	return profile.needsContainer, nil
}

// BuildRunPlan consulta el estado real del contenedor y decide que hace falta.
// No ejecuta nada que cambie el proyecto: solo sondas de lectura.
func BuildRunPlan(ctx context.Context, opts RunOptions) (RunPlan, error) {
	stack := strings.ToLower(strings.TrimSpace(opts.Stack))
	profile, ok := stackProfiles()[stack]
	if !ok {
		return RunPlan{}, fmt.Errorf(
			"coverage --run does not support stack %q yet; supported stacks: %s",
			opts.Stack, strings.Join(SupportedRunStacks(), ", "))
	}
	if strings.TrimSpace(opts.ProjectRoot) == "" {
		return RunPlan{}, fmt.Errorf("project root is required")
	}

	reportName := ManagedReportPrefix + profile.reportExt
	plan := RunPlan{
		Stack:      stack,
		ReportPath: filepath.Join(opts.ProjectRoot, reportName),
	}

	if !profile.needsContainer {
		plan.WorkingDir = opts.ProjectRoot
		plan.Steps = []Step{{
			Title:   "tests",
			Reason:  testCommandReason(opts.TestCommand),
			Command: hostShell(testCommand(opts, profile, reportName)),
			Dir:     opts.ProjectRoot,
		}}

		return plan, nil
	}

	plan.Service = strings.TrimSpace(opts.Service)
	if plan.Service == "" {
		plan.Service = profile.service
	}
	if len(opts.ComposeArgs) == 0 {
		return RunPlan{}, fmt.Errorf("compose arguments are required to reach the %s container", plan.Service)
	}

	probe := containerProbe{opts: opts, service: plan.Service}
	state, err := probe.inspect(ctx)
	if err != nil {
		return RunPlan{}, err
	}
	plan.WorkingDir = state.workingDir

	plan.Steps = append(plan.Steps, driverStep(opts, plan.Service, state))
	plan.Steps = append(plan.Steps, memoryStep(opts, plan.Service, state))
	plan.Steps = append(plan.Steps, Step{
		Title:   "tests",
		Reason:  testCommandReason(opts.TestCommand),
		Command: composeExec(opts, plan.Service, "", testCommand(opts, profile, reportName)),
	})

	return plan, nil
}

func testCommand(opts RunOptions, profile stackProfile, reportName string) string {
	if declared := strings.TrimSpace(opts.TestCommand); declared != "" {
		return declared
	}

	return profile.testCommand(reportName)
}

func testCommandReason(declared string) string {
	if strings.TrimSpace(declared) != "" {
		return "declared in .devherd.yml"
	}

	return "stack default; declare test.command in .devherd.yml to override"
}

// containerState es lo que se necesita saber del contenedor antes de decidir.
type containerState struct {
	workingDir  string
	isRoot      bool
	hasDriver   bool
	memoryLimit int64
	memoryText  string
}

type containerProbe struct {
	opts    RunOptions
	service string
}

func (p containerProbe) inspect(ctx context.Context) (containerState, error) {
	// Una sola llamada para las cuatro sondas: cada `compose exec` cuesta cerca de
	// un segundo, y son de solo lectura.
	const script = `printf '%s\n' "$(pwd)" "$(id -u)" ` +
		`"$(php -m | grep -qix pcov && echo yes || echo no)" ` +
		`"$(php -r 'echo ini_get("memory_limit");')"`

	output, err := p.run(ctx, composeExec(p.opts, p.service, "", script))
	if err != nil {
		return containerState{}, fmt.Errorf(
			"could not inspect the %s container; is the project running? (devherd up): %w", p.service, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 4 {
		return containerState{}, fmt.Errorf("unexpected probe output from the %s container: %q", p.service, output)
	}

	state := containerState{
		workingDir: strings.TrimSpace(lines[0]),
		isRoot:     strings.TrimSpace(lines[1]) == "0",
		hasDriver:  strings.TrimSpace(lines[2]) == "yes",
		memoryText: strings.TrimSpace(lines[3]),
	}
	state.memoryLimit, _ = parseMemoryLimit(state.memoryText)

	return state, nil
}

func (p containerProbe) run(ctx context.Context, args []string) (string, error) {
	commandRunner := p.opts.Runner
	if commandRunner == nil {
		commandRunner = runner.Cmd{}
	}

	return commandRunner.Run(ctx, p.opts.ProjectRoot, args[0], args[1:]...)
}

func driverStep(opts RunOptions, service string, state containerState) Step {
	if state.hasDriver {
		return Step{
			Title:   "coverage driver",
			Reason:  "pcov already present",
			Skipped: true,
		}
	}

	// El `-u root` va solo cuando hace falta: aang-server corre como uid 1000 y
	// `pecl install` falla por permisos, pero tl-mas-server ya es root y forzarlo
	// cambiaria el dueno de lo que escriba.
	user := ""
	reason := "pcov missing; installing it (lost when the container is recreated)"
	if !state.isRoot {
		user = "root"
		reason = "pcov missing; installing as root because the container runs as another user"
	}

	const install = `(php -m | grep -qix pcov || (pecl install pcov && docker-php-ext-enable pcov))`

	return Step{
		Title:   "coverage driver",
		Reason:  reason,
		Command: composeExec(opts, service, user, install),
	}
}

func memoryStep(opts RunOptions, service string, state containerState) Step {
	if state.memoryLimit < 0 {
		return Step{
			Title:   "memory limit",
			Reason:  "unlimited (" + state.memoryText + ")",
			Skipped: true,
		}
	}
	if state.memoryLimit >= minMemoryLimit {
		return Step{
			Title:   "memory limit",
			Reason:  state.memoryText + " is enough",
			Skipped: true,
		}
	}

	user := ""
	if !state.isRoot {
		user = "root"
	}

	// Se escribe un ini y no se pasa `-d`: el runner corre en un proceso hijo que
	// no hereda los flags del padre.
	write := fmt.Sprintf("echo 'memory_limit=%s' > %s", raisedMemoryLimit, memoryLimitFile)

	return Step{
		Title:   "memory limit",
		Reason:  fmt.Sprintf("%s is below %s; pcov needs more headroom", state.memoryText, raisedMemoryLimit),
		Command: composeExec(opts, service, user, write),
	}
}

func composeExec(opts RunOptions, service, user, script string) []string {
	args := append([]string{}, opts.ComposeArgs...)
	args = append(args, "exec", "-T")
	if user != "" {
		args = append(args, "-u", user)
	}
	args = append(args, service, "sh", "-c", script)

	return args
}

func hostShell(script string) []string {
	return []string{"sh", "-c", script}
}

// parseMemoryLimit lee el formato de PHP: un entero opcionalmente sufijado con
// K, M o G, y -1 para sin limite.
func parseMemoryLimit(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if value == "-1" {
		return -1, true
	}

	multiplier := int64(1)
	switch strings.ToUpper(value[len(value)-1:]) {
	case "K":
		multiplier = 1024
		value = value[:len(value)-1]
	case "M":
		multiplier = 1024 * 1024
		value = value[:len(value)-1]
	case "G":
		multiplier = 1024 * 1024 * 1024
		value = value[:len(value)-1]
	}

	number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}

	return number * multiplier, true
}
