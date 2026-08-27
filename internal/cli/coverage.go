package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/devherd/devherd/internal/coverage"
	"github.com/spf13/cobra"
)

// defaultCoverageTop acota la lista de archivos. El resumen tiene que caber en
// una pantalla; para el detalle completo esta --all.
const defaultCoverageTop = 10

func newCoverageCmd() *cobra.Command {
	var (
		report    string
		all       bool
		asJSON    bool
		top       int
		run       bool
		explain   bool
		structure bool
		stack     string
		service   string
	)

	cmd := &cobra.Command{
		Use:   "coverage [project-or-path]",
		Short: "Summarize a coverage report and show where the uncovered mass is",
		Long: "Reads a coverage report produced by the project's own tooling and " +
			"summarizes it. Supported formats: " + strings.Join(coverage.SupportedFormats(), ", ") + ".\n\n" +
			"DevHerd does not instrument code: generate the report with your stack's " +
			"tooling (go test -coverprofile, phpunit --coverage-clover, vitest --coverage, " +
			"jacoco, coverage xml) and pass it with --report.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view := coverageViewOptions{All: all, Top: top}

			if run || explain {
				target := ""
				if len(args) > 0 {
					target = args[0]
				}

				return runCoverage(cmd, coverageRunFlags{
					Target:    target,
					Stack:     stack,
					Service:   service,
					Explain:   explain,
					AsJSON:    asJSON,
					Structure: structure,
					View:      view,
				})
			}

			// El root se resuelve siempre: lo necesita el descubrimiento para saber
			// donde buscar, y --structure para encontrar el go.mod.
			root, err := coverageProjectRoot(firstArg(args))
			if err != nil {
				return err
			}

			reportPath := strings.TrimSpace(report)
			if reportPath == "" {
				// --report explicito manda sobre el autodescubrimiento y no busca nada.
				discovery, err := coverage.DiscoverReport(root, coverageStackOrEmpty(root))
				if err != nil {
					return err
				}
				// Decir que archivo se uso no es cortesia: sin eso, leer el reporte del
				// front creyendo que es el del back no deja ninguna señal.
				writeCoverageDiscovery(cmd.OutOrStdout(), discovery)
				reportPath = discovery.Chosen.Path
			}

			parsed, err := coverage.ParseFile(reportPath)
			if err != nil {
				return err
			}

			if structure {
				// El root no sale del reporte, que puede estar en /tmp: sale del
				// proyecto, que es donde vive el go.mod que traduce las rutas.
				view.Source = reportPath

				return runCoverageStructure(cmd.OutOrStdout(), parsed, coverageStructureFlags{
					Root:   root,
					Source: reportPath,
					View:   view,
					AsJSON: asJSON,
				})
			}

			if asJSON {
				return writeCoverageJSON(cmd.OutOrStdout(), parsed)
			}

			view.Source = reportPath
			writeCoverageReport(cmd.OutOrStdout(), parsed, view)

			return nil
		},
	}

	cmd.Flags().StringVar(&report, "report", "",
		"Path to an existing coverage report. Without it, the report is discovered by the project's stack")
	cmd.Flags().BoolVar(&run, "run", false, "Prepare the project container, run its tests and read the result")
	cmd.Flags().BoolVar(&explain, "explain", false, "Print the commands --run would execute, without running any")
	cmd.Flags().BoolVar(&structure, "structure", false,
		"Attribute coverage to functions and report the ceiling the code structure imposes (Go only)")
	cmd.Flags().StringVar(&stack, "stack", "", "Override the detected stack (laravel, go)")
	cmd.Flags().StringVar(&service, "service", "", "Compose service to run the tests in")
	cmd.Flags().BoolVar(&all, "all", false, "List every file instead of the largest uncovered ones")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the parsed report as JSON")
	cmd.Flags().IntVar(&top, "top", defaultCoverageTop, "How many rows to list when not using --all")

	return cmd
}

type coverageViewOptions struct {
	Source string
	All    bool
	Top    int
}

func writeCoverageJSON(out io.Writer, report coverage.Report) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode coverage report: %w", err)
	}

	return nil
}

func writeCoverageReport(out io.Writer, report coverage.Report, opts coverageViewOptions) {
	// La unidad va en la cabecera siempre. Sin ella, comparar el 58% de un
	// proyecto Go con el 58% de uno JS parece razonable, y no lo es: uno cuenta
	// sentencias y el otro lineas.
	fmt.Fprintf(out, "%s  ·  %s  ·  %s\n\n", filepath.Base(opts.Source), report.Format, report.Unit)

	if report.IsEmpty() {
		// Un 0% aqui se leeria como "nada esta probado", que es lo contrario de
		// "no hay nada medido".
		fmt.Fprintln(out, "  no coverage data: the report contains no measurable units")

		return
	}

	writeCoverageGroups(out, report, opts)

	fmt.Fprintf(out, "\n  %-38s %6.1f%%   (%d %s)\n",
		"total", report.Percent(), report.Total(), report.Unit)

	writeCoverageFiles(out, report, opts)
}

func writeCoverageGroups(out io.Writer, report coverage.Report, opts coverageViewOptions) {
	groups := report.Groups()
	if len(groups) <= 1 {
		return
	}

	// Vienen ordenados por masa sin cubrir, asi que la cola son directorios ya
	// cubiertos: informacion cero. Se acotan igual que la lista de archivos, o la
	// tabla principal volcaria 38 filas mientras los archivos se limitan a 10.
	limit := len(groups)
	if !opts.All {
		limit = opts.Top
		if limit <= 0 {
			limit = defaultCoverageTop
		}
		if limit > len(groups) {
			limit = len(groups)
		}
	}

	fmt.Fprintf(out, "  %-38s %7s %12s\n", "directory", "covered", "units")
	for _, group := range groups[:limit] {
		fmt.Fprintf(out, "  %-38s %6.1f%% %12s\n",
			truncateCoveragePath(group.Name, 38),
			group.Percent(),
			fmt.Sprintf("%d/%d", group.Covered, group.Total))
	}

	if remaining := len(groups) - limit; remaining > 0 {
		fmt.Fprintf(out, "  %d more director%s (--all to list them)\n",
			remaining, map[bool]string{true: "y", false: "ies"}[remaining == 1])
	}
}

func writeCoverageFiles(out io.Writer, report coverage.Report, opts coverageViewOptions) {
	files := report.ByUncovered()
	if opts.All {
		files = report.ByPath()
		fmt.Fprintf(out, "\n  Files (%d):\n", len(files))
		for _, file := range files {
			writeCoverageFileLine(out, file)
		}

		return
	}

	// Ordenado por masa sin cubrir y no por porcentaje: es lo que dice donde
	// trabajar. Un archivo de 800 unidades al 40% deja 480 sin cubrir; uno de 3
	// al 0% deja 3, y por porcentaje saldria primero.
	if report.Uncovered() == 0 {
		fmt.Fprintln(out, "\n  Every measured unit is covered.")

		return
	}

	limit := opts.Top
	if limit <= 0 {
		limit = defaultCoverageTop
	}

	shown := 0
	fmt.Fprintln(out, "\n  Largest uncovered mass:")
	for _, file := range files {
		if file.Uncovered() == 0 || shown == limit {
			break
		}
		writeCoverageFileLine(out, file)
		shown++
	}

	// Nunca truncar en silencio: quien no ve la nota asume que vio todo.
	if remaining := countCoverageUncoveredFiles(files) - shown; remaining > 0 {
		fmt.Fprintf(out, "    %d more file(s) with uncovered %s (--all to list them)\n", remaining, report.Unit)
	}
}

func writeCoverageFileLine(out io.Writer, file coverage.FileReport) {
	fmt.Fprintf(out, "    %-46s %6d uncovered %6.1f%%\n",
		truncateCoveragePath(file.Path, 46), file.Uncovered(), file.Percent())
}

func countCoverageUncoveredFiles(files []coverage.FileReport) int {
	count := 0
	for _, file := range files {
		if file.Uncovered() > 0 {
			count++
		}
	}

	return count
}

// truncateCoveragePath recorta por la izquierda: en una ruta larga lo que
// identifica al archivo esta al final, no al principio.
func truncateCoveragePath(value string, limit int) string {
	if limit <= 3 || len(value) <= limit {
		return value
	}

	return "..." + value[len(value)-(limit-3):]
}

// firstArg devuelve el argumento posicional si lo hay. El comando lo trata como
// opcional en los dos caminos, y repetir el chequeo en cada uno invita a que se
// desincronicen.
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[0]
}

// coverageStackOrEmpty detecta el stack sin fallar si no lo reconoce. El
// descubrimiento sabe buscar todas las convenciones cuando no hay stack, y exigir
// que el proyecto sea identificable seria mas estricto que el problema: lo que se
// busca es un archivo, no un ecosistema.
func coverageStackOrEmpty(root string) string {
	stack, err := coverageStack("", root)
	if err != nil {
		return ""
	}

	return stack
}

// writeCoverageDiscovery dice de donde salio el reporte y que mas habia. Elegir en
// silencio entre dos reportes es como se lee una medicion que no era la buscada.
func writeCoverageDiscovery(out io.Writer, discovery coverage.Discovery) {
	origin := discovery.Chosen.RelPath
	switch {
	case discovery.Chosen.Managed:
		origin += "  (written by devherd coverage --run)"
	case discovery.Chosen.Stack != "":
		origin += "  (" + discovery.Chosen.Stack + " convention)"
	}
	fmt.Fprintf(out, "using %s\n", origin)

	for _, other := range discovery.Others {
		note := other.RelPath
		switch {
		case other.Managed:
			note += "  (written by devherd coverage --run)"
		case other.Stack != "":
			note += "  (" + other.Stack + " convention)"
		}
		fmt.Fprintf(out, "  also found, not used: %s\n", note)
	}
	fmt.Fprintln(out)
}
