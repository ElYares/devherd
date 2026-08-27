package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/devherd/devherd/internal/coverage"
)

// coverageStructureFlags es lo que el analisis estructural necesita del comando.
type coverageStructureFlags struct {
	// Root es cualquier ruta dentro del modulo analizado. El perfil de Go nombra
	// los archivos por import path, y sin el go.mod no hay como llegar al fuente.
	Root   string
	Source string
	View   coverageViewOptions
	AsJSON bool
}

func runCoverageStructure(out io.Writer, report coverage.Report, flags coverageStructureFlags) error {
	structure, err := coverage.Structure(report, flags.Root)
	if err != nil {
		var unsupported coverage.ErrNoStructuralAnalysis
		if errors.As(err, &unsupported) {
			// No es un fallo del comando: es un limite del formato, y decirlo es lo
			// unico honesto. Inventar un techo para un reporte que no trae rangos de
			// linea seria peor que no ofrecer la funcion.
			fmt.Fprintf(out, "%s\n\n", unsupported.Error())
			fmt.Fprintln(out, "  run without --structure for the per-file summary")

			return nil
		}

		return err
	}

	if flags.AsJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(structure); err != nil {
			return fmt.Errorf("encode structural report: %w", err)
		}

		return nil
	}

	writeCoverageStructure(out, structure, flags)

	return nil
}

func writeCoverageStructure(out io.Writer, structure coverage.StructureReport, flags coverageStructureFlags) {
	fmt.Fprintf(out, "%s  ·  %s  ·  %s  ·  structure\n\n",
		filepath.Base(flags.Source), structure.Format, structure.Unit)

	if structure.IsEmpty() {
		fmt.Fprintln(out, "  no functions could be attributed: the report has no measurable units")

		return
	}

	writeCoverageCeiling(out, structure)
	writeCoverageFunctions(out, structure, flags.View)
	writeCoverageStructureCaveats(out, structure)
}

// writeCoverageCeiling es el corazon del comando: los tres numeros que convierten
// un porcentaje suelto en una decision.
func writeCoverageCeiling(out io.Writer, structure coverage.StructureReport) {
	fmt.Fprintf(out, "  %-38s %6.1f%% %12s\n", "covered",
		structure.Percent(), fmt.Sprintf("%d/%d", structure.Covered(), structure.Total()))
	fmt.Fprintf(out, "  %-38s %6.1f%% %12s\n", "reachable ceiling",
		structure.Ceiling(), fmt.Sprintf("%d/%d", structure.ReachableTotal(), structure.Total()))
	fmt.Fprintf(out, "  %-38s %6.1f%% %12s\n", "covered of what is reachable",
		structure.PercentOfReachable(),
		fmt.Sprintf("%d/%d", structure.ReachableCovered(), structure.ReachableTotal()))

	if stored := structure.StoredTotal(); stored > 0 {
		// Sin esta linea, el techo parece un limite arbitrario. Nombrar de donde
		// sale es lo que permite decidir si el refactor vale la pena.
		fmt.Fprintf(out,
			"\n  %d %s live in closures stored into data structures (RunE and friends).\n"+
				"  Testing them means wiring the value up, not writing more test cases.\n",
			stored, structure.Unit)
	}
}

func writeCoverageFunctions(out io.Writer, structure coverage.StructureReport, opts coverageViewOptions) {
	functions := structure.ByUncovered()
	// Las cubiertas del todo no dicen donde trabajar, y son la mayoria en un
	// proyecto sano: la lista se ocupa de las que dejan masa.
	pending := make([]coverage.FunctionReport, 0, len(functions))
	for _, fn := range functions {
		if fn.Uncovered() > 0 {
			pending = append(pending, fn)
		}
	}

	if len(pending) == 0 {
		fmt.Fprintln(out, "\n  every attributed function is fully covered")

		return
	}

	limit := len(pending)
	if !opts.All {
		limit = opts.Top
		if limit <= 0 {
			limit = defaultCoverageTop
		}
		if limit > len(pending) {
			limit = len(pending)
		}
	}

	fmt.Fprintf(out, "\n  %-44s %-9s %8s %9s\n", "function", "kind", "missing", "covered")
	for _, fn := range pending[:limit] {
		kind := string(fn.Kind)
		if !fn.Reachable() {
			// La marca va en la fila y no en una leyenda al pie: es el dato que
			// cambia que hacer con esa funcion.
			kind = "stored"
		}
		fmt.Fprintf(out, "  %-44s %-9s %8d %8.1f%%\n",
			truncateCoveragePath(fn.Name, 44), kind, fn.Uncovered(), fn.Percent())
	}

	if remaining := len(pending) - limit; remaining > 0 {
		fmt.Fprintf(out, "  %d more function%s with uncovered mass (--all to list them)\n",
			remaining, map[bool]string{true: "", false: "s"}[remaining == 1])
	}
}

// writeCoverageStructureCaveats dice que quedo fuera del analisis. Un analisis
// parcial presentado como completo es peor que no tenerlo.
func writeCoverageStructureCaveats(out io.Writer, structure coverage.StructureReport) {
	if count := len(structure.Unresolved); count > 0 {
		fmt.Fprintf(out, "\n  %d file%s could not be read from disk and %s excluded:\n",
			count,
			map[bool]string{true: "", false: "s"}[count == 1],
			map[bool]string{true: "was", false: "were"}[count == 1])
		shown := structure.Unresolved
		if len(shown) > 3 {
			shown = shown[:3]
		}
		for _, path := range shown {
			fmt.Fprintf(out, "    %s\n", path)
		}
		if remaining := count - len(shown); remaining > 0 {
			fmt.Fprintf(out, "    and %d more\n", remaining)
		}
	}

	if structure.OrphanStmts > 0 {
		fmt.Fprintf(out,
			"\n  %d %s fell outside every function: the profile and the sources disagree.\n"+
				"  Regenerate the report against the current tree.\n",
			structure.OrphanStmts, structure.Unit)
	}
}
