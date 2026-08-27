package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/coverage"
	"github.com/devherd/devherd/internal/detector"
	"github.com/spf13/cobra"
)

type coverageRunFlags struct {
	Target  string
	Stack   string
	Service string
	Explain bool
	AsJSON  bool
	View    coverageViewOptions
}

func runCoverage(cmd *cobra.Command, flags coverageRunFlags) error {
	app, err := loadAppContext(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { _ = app.DB.Close() }()

	root, err := coverageProjectRoot(flags.Target)
	if err != nil {
		return err
	}

	// El stack se resuelve antes que compose: un proyecto Go no tiene archivos
	// compose, y pedirselos lo dejaba fuera del comando aunque no necesite
	// contenedor para nada.
	stack, err := coverageStack(flags.Stack, root)
	if err != nil {
		return err
	}

	needsContainer, err := coverage.StackNeedsContainer(stack)
	if err != nil {
		return err
	}

	options := coverage.RunOptions{Stack: stack, ProjectRoot: root, Service: flags.Service}

	project, composeErr := prepareComposeProject(cmd.Context(), app, flags.Target)
	switch {
	case composeErr != nil && needsContainer:
		return composeErr
	case composeErr == nil:
		options.ProjectRoot = project.Root
		options.ComposeArgs = compose.Command(project)
		options.TestCommand = project.Test.Command
		options.Service = firstNonEmptyCoverage(flags.Service, project.Test.Service)
	}

	plan, err := coverage.BuildRunPlan(cmd.Context(), options)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  (%s)", filepath.Base(options.ProjectRoot), plan.Stack)
	if plan.Service != "" {
		fmt.Fprintf(out, "  ·  service %s", plan.Service)
	}
	fmt.Fprintln(out)

	if flags.Explain {
		coverage.ExplainRunPlan(out, plan)

		return nil
	}

	fmt.Fprintln(out)
	result, runErr := coverage.ExecuteRunPlan(cmd.Context(), plan, nil, out)

	// Un fallo de pruebas no oculta el numero: la corrida ya costo su tiempo y la
	// cobertura de lo que si corrio es real. Lo que cambia es que el aviso va
	// **antes** del resumen, y que se sale en rojo.
	if runErr != nil && !errors.Is(runErr, coverage.ErrTestsFailed) {
		return runErr
	}

	if runErr != nil {
		writeCoverageTestFailure(cmd.ErrOrStderr(), result)
	}

	fmt.Fprintln(out)
	if flags.AsJSON {
		if err := writeCoverageJSON(out, result.Report); err != nil {
			return err
		}
	} else {
		flags.View.Source = plan.ReportPath
		writeCoverageReport(out, result.Report, flags.View)
	}

	warnCoverageReportNotIgnored(cmd.ErrOrStderr(), options.ProjectRoot, plan.ReportPath)

	return runErr
}

// coverageProjectRoot resuelve la raiz sin depender de compose.
func coverageProjectRoot(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		root, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current directory: %w", err)
		}

		return root, nil
	}

	root, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}

	return root, nil
}

// coverageStack usa lo declarado si lo hay, y si no detecta el framework del
// proyecto. Adivinar mal es barato de corregir con --stack; obligar a declararlo
// siempre no lo es.
func coverageStack(declared, root string) (string, error) {
	if trimmed := strings.TrimSpace(declared); trimmed != "" {
		return strings.ToLower(trimmed), nil
	}

	detected, found, err := detector.DetectProject(root)
	if err != nil {
		return "", fmt.Errorf("detect project stack: %w", err)
	}
	if !found || strings.TrimSpace(detected.Framework) == "" {
		return "", fmt.Errorf(
			"could not detect the stack of %s; pass --stack (%s)",
			root, strings.Join(coverage.SupportedRunStacks(), ", "))
	}

	return strings.ToLower(detected.Framework), nil
}

func writeCoverageTestFailure(out io.Writer, result coverage.RunResult) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  WARNING: the project tests failed.")
	fmt.Fprintln(out, "  The numbers below cover only what ran; treat them as a floor, not a measurement.")
	if trimmed := strings.TrimSpace(result.TestOutput); trimmed != "" {
		fmt.Fprintln(out)
		for _, line := range lastLines(trimmed, 10) {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
}

// warnCoverageReportNotIgnored avisa una sola vez. El reporte tiene que vivir
// dentro del proyecto —el contenedor solo ve eso montado—, asi que lo unico que se
// puede hacer es que no acabe commiteado por descuido.
func warnCoverageReportNotIgnored(out io.Writer, root, reportPath string) {
	name := filepath.Base(reportPath)

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == name || line == coverage.ManagedReportPrefix+"*" || line == "/"+name {
				return
			}
		}
	} else if !os.IsNotExist(err) {
		return
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return
	}

	fmt.Fprintf(out, "\nnote: %s is not in .gitignore; add `%s*` to keep it out of commits\n",
		name, coverage.ManagedReportPrefix)
}

func lastLines(value string, count int) []string {
	lines := strings.Split(value, "\n")
	if len(lines) <= count {
		return lines
	}

	return lines[len(lines)-count:]
}

func firstNonEmptyCoverage(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
