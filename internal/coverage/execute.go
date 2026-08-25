package coverage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devherd/devherd/internal/runner"
)

// ErrTestsFailed marca que las pruebas del proyecto salieron en rojo. Se
// distingue porque cambia como se presenta el resultado, no si se presenta: el
// numero se muestra igual, con el aviso por delante.
var ErrTestsFailed = errors.New("project tests failed")

// ErrNoReport marca que la corrida no dejo reporte. Es distinto de un reporte sin
// unidades: aquel dice "no hay nada medido", este dice "no llego a medirse".
var ErrNoReport = errors.New("the test run produced no coverage report")

// RunResult es lo que quedo despues de ejecutar un plan.
type RunResult struct {
	Report Report
	// TestOutput es la salida de las pruebas, que solo interesa cuando fallan.
	TestOutput string
	Failed     bool
}

// ExecuteRunPlan corre los pasos del plan e informa de cada uno mientras avanza.
// Los pasos marcados como Skipped se anuncian y no se ejecutan.
func ExecuteRunPlan(ctx context.Context, plan RunPlan, commandRunner runner.Runner, out io.Writer) (RunResult, error) {
	if commandRunner == nil {
		commandRunner = runner.Cmd{}
	}

	// El reporte anterior se borra antes de empezar. Si no, una corrida que muere
	// sin generar nada dejaria leer el de la vez pasada como si fuera de ahora.
	if err := os.Remove(plan.ReportPath); err != nil && !os.IsNotExist(err) {
		return RunResult{}, fmt.Errorf("remove previous coverage report: %w", err)
	}

	var result RunResult
	for _, step := range plan.Steps {
		if step.Skipped {
			fmt.Fprintf(out, "  · %-16s %s\n", step.Title, step.Reason)
			continue
		}

		dir := step.Dir
		output, err := commandRunner.Run(ctx, dir, step.Command[0], step.Command[1:]...)
		if step.Title != "tests" {
			if err != nil {
				return result, fmt.Errorf("%s: %w", step.Title, err)
			}
			fmt.Fprintf(out, "  ✓ %-16s %s\n", step.Title, step.Reason)

			continue
		}

		result.TestOutput = output
		if err != nil {
			// Unas pruebas en rojo no abortan la lectura: la cobertura de lo que si
			// corrio es real, y la corrida ya costo su tiempo.
			result.Failed = true
			result.TestOutput = joinOutput(output, err.Error())
			fmt.Fprintf(out, "  ✗ %-16s the project tests failed\n", step.Title)

			continue
		}

		fmt.Fprintf(out, "  ✓ %-16s %s\n", step.Title, summarizeTestOutput(output))
	}

	report, err := ParseFile(plan.ReportPath)
	if err != nil {
		if os.IsNotExist(errors.Unwrap(err)) || strings.Contains(err.Error(), "no such file") {
			if result.Failed {
				return result, fmt.Errorf("%w, and %w", ErrTestsFailed, ErrNoReport)
			}

			return result, ErrNoReport
		}

		return result, err
	}

	result.Report = report
	if result.Failed {
		return result, ErrTestsFailed
	}

	return result, nil
}

// summarizeTestOutput busca la linea de conteo del runner, no la ultima: en Pest
// la ultima es "Tests: 865 passed", pero en `go test` es un "?" de un paquete sin
// pruebas, que no dice nada. Volcar la salida entera tapa el resumen de cobertura,
// que es lo que se vino a ver.
func summarizeTestOutput(output string) string {
	markers := []string{"Tests:", "passed", "OK (", "Assertions:"}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(stripANSI(lines[i]))
		if line == "" {
			continue
		}

		for _, marker := range markers {
			if strings.Contains(line, marker) {
				return truncateRunText(line, 60)
			}
		}
	}

	return "passed"
}

// stripANSI quita los codigos de color: Pest y PHPUnit los emiten aunque la salida
// no sea una terminal, y sin limpiarlos el resumen sale ilegible.
func stripANSI(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != 0x1b {
			builder.WriteByte(value[i])
			continue
		}

		// Saltar hasta el final de la secuencia, que termina en una letra.
		for i < len(value) && !isANSITerminator(value[i]) {
			i++
		}
	}

	return builder.String()
}

func isANSITerminator(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func truncateRunText(value string, limit int) string {
	if limit <= 3 || len(value) <= limit {
		return value
	}

	return strings.TrimSpace(value[:limit-3]) + "..."
}

func joinOutput(left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return left + "\n" + right
	}
}

// ExplainRunPlan imprime los comandos exactos sin ejecutar ninguno. Existe porque
// automatizar algo no deberia volverlo opaco: con esto se puede leer lo que va a
// pasar, o copiar los comandos y correrlos a mano.
func ExplainRunPlan(out io.Writer, plan RunPlan) {
	for _, step := range plan.Steps {
		fmt.Fprintf(out, "\n# %s — %s\n", step.Title, step.Reason)
		if step.Skipped {
			fmt.Fprintln(out, "# (nothing to do)")

			continue
		}

		if step.Dir != "" {
			fmt.Fprintf(out, "cd %s\n", step.Dir)
		}
		fmt.Fprintln(out, shellQuote(step.Command))
	}

	fmt.Fprintf(out, "\n# read the result\ndevherd coverage --report %s\n", plan.ReportPath)
}

// shellQuote arma una linea copiable y pegable. Solo entrecomilla lo que lo
// necesita, para que el comando siga siendo legible.
func shellQuote(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "" && !strings.ContainsAny(arg, " \t\n'\"$`\\|&;<>()*?") {
			quoted = append(quoted, arg)

			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
	}

	return strings.Join(quoted, " ")
}
