package coverage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedRunner ejecuta un plan simulando el resultado de cada paso, y puede
// escribir el reporte como efecto de correr las pruebas.
type scriptedRunner struct {
	testOutput string
	testErr    error
	// writeReport es lo que dejaria la suite en disco. Vacio simula una corrida que
	// murio sin generar nada.
	writeReport string
	reportPath  string
	calls       []string
}

func (r *scriptedRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, command)

	if !strings.Contains(command, "artisan test") && !strings.Contains(command, "go test") {
		return "", nil
	}

	if r.writeReport != "" {
		if err := os.WriteFile(r.reportPath, []byte(r.writeReport), 0o644); err != nil {
			return "", err
		}
	}

	return r.testOutput, r.testErr
}

const cloverForRun = `<?xml version="1.0"?>
<coverage><project>
  <file name="app/Demo.php">
    <line num="1" type="stmt" count="1"/>
    <line num="2" type="stmt" count="0"/>
  </file>
</project></coverage>`

func planForTest(t *testing.T) (RunPlan, string) {
	t.Helper()

	root := t.TempDir()
	reportPath := filepath.Join(root, ManagedReportPrefix+".xml")

	return RunPlan{
		Stack:      "laravel",
		Service:    "app",
		ReportPath: reportPath,
		Steps: []Step{
			{Title: "coverage driver", Reason: "pcov already present", Skipped: true},
			{Title: "memory limit", Reason: "512M is enough", Skipped: true},
			{Title: "tests", Command: []string{"docker", "compose", "exec", "-T", "app", "sh", "-c", "php artisan test"}},
		},
	}, reportPath
}

func TestExecuteRunPlanReadsTheReportItProduced(t *testing.T) {
	plan, reportPath := planForTest(t)
	runner := &scriptedRunner{
		testOutput:  "  Tests:    865 passed (2203 assertions)",
		writeReport: cloverForRun,
		reportPath:  reportPath,
	}

	var out bytes.Buffer
	result, err := ExecuteRunPlan(context.Background(), plan, runner, &out)
	if err != nil {
		t.Fatalf("ExecuteRunPlan returned error: %v", err)
	}

	if result.Report.Total() != 2 || result.Report.Covered() != 1 {
		t.Fatalf("expected 1/2 statements, got %d/%d", result.Report.Covered(), result.Report.Total())
	}
	// Los pasos saltados se anuncian igual: saber que algo ya estaba resuelto vale
	// tanto como haberlo hecho.
	if !strings.Contains(out.String(), "pcov already present") {
		t.Fatalf("expected skipped steps announced:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "865 passed") {
		t.Fatalf("expected the test counts summarized:\n%s", out.String())
	}
}

// Decision: unas pruebas en rojo no ocultan el numero. La corrida ya costo su
// tiempo y la cobertura de lo que si corrio es real; lo que cambia es el aviso.
func TestExecuteRunPlanKeepsTheNumbersWhenTestsFail(t *testing.T) {
	plan, reportPath := planForTest(t)
	runner := &scriptedRunner{
		testOutput:  "Tests: 3 failed, 100 passed",
		testErr:     errors.New("exit status 1"),
		writeReport: cloverForRun,
		reportPath:  reportPath,
	}

	var out bytes.Buffer
	result, err := ExecuteRunPlan(context.Background(), plan, runner, &out)

	if !errors.Is(err, ErrTestsFailed) {
		t.Fatalf("expected ErrTestsFailed, got %v", err)
	}
	if !result.Failed {
		t.Fatal("the result must be marked as failed")
	}
	if result.Report.Total() == 0 {
		t.Fatal("the coverage of what did run is real and must survive")
	}
	if !strings.Contains(result.TestOutput, "3 failed") {
		t.Fatalf("the test output must be kept for the warning, got %q", result.TestOutput)
	}
}

// Una suite que muere sin generar reporte es distinto de un reporte vacio: aquel
// dice "no llego a medirse", este dice "no hay nada medido".
func TestExecuteRunPlanDistinguishesNoReportFromEmptyReport(t *testing.T) {
	plan, reportPath := planForTest(t)
	runner := &scriptedRunner{
		testOutput: "Fatal error: Allowed memory size exhausted",
		testErr:    errors.New("exit status 255"),
		reportPath: reportPath,
	}

	var out bytes.Buffer
	_, err := ExecuteRunPlan(context.Background(), plan, runner, &out)

	if !errors.Is(err, ErrNoReport) {
		t.Fatalf("expected ErrNoReport, got %v", err)
	}
	if !errors.Is(err, ErrTestsFailed) {
		t.Fatalf("the failure should also be reported, got %v", err)
	}
}

// Sin borrar el anterior, una corrida que muere dejaria leer el reporte de la vez
// pasada como si fuera de ahora. Es la peor clase de mentira: plausible.
func TestExecuteRunPlanRemovesTheStaleReportBeforeRunning(t *testing.T) {
	plan, reportPath := planForTest(t)
	if err := os.WriteFile(reportPath, []byte(cloverForRun), 0o644); err != nil {
		t.Fatalf("seed stale report: %v", err)
	}

	runner := &scriptedRunner{
		testErr:    errors.New("exit status 255"),
		reportPath: reportPath,
	}

	var out bytes.Buffer
	_, err := ExecuteRunPlan(context.Background(), plan, runner, &out)
	if !errors.Is(err, ErrNoReport) {
		t.Fatalf("the stale report must not be read as fresh; got %v", err)
	}
	if _, statErr := os.Stat(reportPath); !os.IsNotExist(statErr) {
		t.Fatal("the stale report should have been removed")
	}
}

// Un fallo preparando el contenedor si aborta: sin driver no hay nada que medir.
func TestExecuteRunPlanStopsWhenPreparationFails(t *testing.T) {
	plan, reportPath := planForTest(t)
	plan.Steps[0] = Step{
		Title:   "coverage driver",
		Command: []string{"docker", "compose", "exec", "-T", "app", "sh", "-c", "pecl install pcov"},
	}

	runner := &scriptedRunner{reportPath: reportPath}
	runner.testErr = nil

	failing := &failingStepRunner{fail: "pecl install"}
	var out bytes.Buffer
	_, err := ExecuteRunPlan(context.Background(), plan, failing, &out)
	if err == nil {
		t.Fatal("expected the run to stop when the driver cannot be installed")
	}
	if !strings.Contains(err.Error(), "coverage driver") {
		t.Fatalf("the error should name the step, got %v", err)
	}
	if failing.ranTests {
		t.Fatal("the tests must not run when preparation failed")
	}
}

type failingStepRunner struct {
	fail     string
	ranTests bool
}

func (r *failingStepRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	if strings.Contains(command, "artisan test") {
		r.ranTests = true
	}
	if strings.Contains(command, r.fail) {
		return "", errors.New("permission denied")
	}

	return "", nil
}

func TestExplainPrintsEveryCommandAndRunsNothing(t *testing.T) {
	plan, _ := planForTest(t)
	plan.Steps[0] = Step{
		Title:   "coverage driver",
		Reason:  "pcov missing",
		Command: []string{"docker", "compose", "exec", "-T", "-u", "root", "app", "sh", "-c", "pecl install pcov"},
	}

	var out bytes.Buffer
	ExplainRunPlan(&out, plan)

	text := out.String()
	for _, want := range []string{"coverage driver", "pcov missing", "pecl install pcov", "artisan test", "devherd coverage --report"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in the explanation:\n%s", want, text)
		}
	}
	// Lo saltado se dice, no se omite.
	if !strings.Contains(text, "nothing to do") {
		t.Fatalf("expected skipped steps marked:\n%s", text)
	}
}

// La linea impresa tiene que poder pegarse en una terminal tal cual.
func TestExplainQuotesTheShellScript(t *testing.T) {
	plan, _ := planForTest(t)

	var out bytes.Buffer
	ExplainRunPlan(&out, plan)

	if !strings.Contains(out.String(), `sh -c 'php artisan test'`) {
		t.Fatalf("expected the script quoted as a single argument:\n%s", out.String())
	}
}

func TestSummarizeTestOutputPicksTheCountLine(t *testing.T) {
	// `go test` termina con una linea que no dice nada; la util esta antes.
	goOutput := "ok  \tgithub.com/x/y\t0.1s\tcoverage: 80.0% of statements\n?   \tgithub.com/x/z\t[no test files]"
	if got := summarizeTestOutput(goOutput); strings.Contains(got, "no test files") {
		t.Fatalf("the last line of go test is not a summary, got %q", got)
	}

	pest := "\x1b[32m  Tests:\x1b[39m    865 passed (2203 assertions)"
	got := summarizeTestOutput(pest)
	if !strings.Contains(got, "865 passed") {
		t.Fatalf("expected the Pest counts, got %q", got)
	}
	// Sin limpiar los codigos de color el resumen sale ilegible.
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ANSI codes must be stripped, got %q", got)
	}
}
