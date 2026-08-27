package coverage

import (
	"fmt"
	"sort"
)

// StructureReport es la cobertura vista por funcion, con el techo que impone la
// estructura del codigo. Es lo que ninguna herramienta de los cinco ecosistemas
// entrega: un porcentaje suelto no distingue entre codigo abandonado y codigo que
// no se puede probar sin refactorizar antes.
type StructureReport struct {
	// Format y Unit vienen del reporte del que se derivo, para que la tabla
	// estructural no pueda presentarse en unidades distintas al resumen.
	Format string `json:"format"`
	Unit   Unit   `json:"unit"`
	// Functions son todas las funciones con al menos una unidad medida.
	Functions []FunctionReport `json:"functions"`
	// Unresolved son los archivos del reporte cuya fuente no se pudo abrir. Se
	// listan en vez de ignorarse: son masa que el analisis no vio, y callarla haria
	// pasar un analisis parcial por completo.
	Unresolved []string `json:"unresolved,omitempty"`
	// OrphanStmts son las unidades de bloques que no cayeron en ninguna funcion.
	OrphanStmts int `json:"orphan_stmts,omitempty"`
}

// Total suma las unidades atribuidas a funciones.
func (s StructureReport) Total() int {
	total := 0
	for _, fn := range s.Functions {
		total += fn.Total
	}

	return total
}

// Covered suma las unidades cubiertas.
func (s StructureReport) Covered() int {
	covered := 0
	for _, fn := range s.Functions {
		covered += fn.Covered
	}

	return covered
}

// Percent es la cobertura tal como la reporta cualquier herramienta: sobre el
// total, sin descontar nada.
func (s StructureReport) Percent() float64 {
	return percent(s.Covered(), s.Total())
}

// ReachableTotal son las unidades que un test puede alcanzar sin refactorizar.
func (s StructureReport) ReachableTotal() int {
	total := 0
	for _, fn := range s.Functions {
		if fn.Reachable() {
			total += fn.Total
		}
	}

	return total
}

// ReachableCovered son las unidades cubiertas dentro de lo alcanzable.
func (s StructureReport) ReachableCovered() int {
	covered := 0
	for _, fn := range s.Functions {
		if fn.Reachable() {
			covered += fn.Covered
		}
	}

	return covered
}

// StoredTotal son las unidades que viven en closures guardados en estructuras de
// datos. Es la masa que exige un refactor antes que un test.
func (s StructureReport) StoredTotal() int {
	return s.Total() - s.ReachableTotal()
}

// Ceiling es el porcentaje maximo que la cobertura puede alcanzar sin cambiar la
// estructura del codigo. Perseguir un numero por encima de este techo es tirar el
// tiempo, y es exactamente lo que pasa cuando solo se mira el porcentaje.
func (s StructureReport) Ceiling() float64 {
	return percent(s.ReachableTotal(), s.Total())
}

// PercentOfReachable es cuanto de lo alcanzable esta cubierto. Es el numero
// honesto para juzgar el esfuerzo invertido: un 21,4% que resulta ser el 68,9% de
// lo alcanzable no es abandono.
func (s StructureReport) PercentOfReachable() float64 {
	return percent(s.ReachableCovered(), s.ReachableTotal())
}

// IsEmpty distingue "no se pudo atribuir nada" de "nada esta cubierto".
func (s StructureReport) IsEmpty() bool {
	return len(s.Functions) == 0
}

// ByUncovered ordena las funciones por masa sin cubrir, de mayor a menor. Por
// porcentaje, una funcion de 3 unidades al 0% se pondria por delante de una de 80
// al 10%, que es el orden inverso al que sirve para decidir donde trabajar.
func (s StructureReport) ByUncovered() []FunctionReport {
	functions := append([]FunctionReport(nil), s.Functions...)
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Uncovered() != functions[j].Uncovered() {
			return functions[i].Uncovered() > functions[j].Uncovered()
		}
		if functions[i].File != functions[j].File {
			return functions[i].File < functions[j].File
		}

		return functions[i].StartLine < functions[j].StartLine
	})

	return functions
}

// ErrNoStructuralAnalysis se devuelve cuando el reporte no trae el detalle que el
// analisis necesita. Es un error con nombre y no un reporte vacio porque la
// respuesta correcta es decir que para ese stack solo hay agregacion por archivo,
// nunca inventar un techo.
type ErrNoStructuralAnalysis struct {
	Format string
}

func (e ErrNoStructuralAnalysis) Error() string {
	return fmt.Sprintf(
		"structural analysis is only available for Go coverage profiles; %s reports "+
			"do not carry per-block line ranges, so only per-file aggregation is possible",
		e.Format)
}

// Structure atribuye un reporte completo a funciones y calcula el techo. root es
// cualquier ruta dentro del modulo que se analiza: se sube desde ahi buscando el
// go.mod que traduce las rutas del perfil.
func Structure(report Report, root string) (StructureReport, error) {
	if !report.HasBlocks() {
		format := report.Format
		if format == "" {
			format = "this"
		}

		return StructureReport{}, ErrNoStructuralAnalysis{Format: format}
	}

	module, err := FindModule(root)
	if err != nil {
		return StructureReport{}, err
	}

	structure := StructureReport{Format: report.Format, Unit: report.Unit}
	for _, file := range report.ByPath() {
		if !file.HasBlocks() {
			continue
		}

		path, ok := module.SourcePath(file.Path)
		if !ok {
			structure.Unresolved = append(structure.Unresolved, file.Path)

			continue
		}

		attribution, err := AttributeFile(file, path)
		if err != nil {
			// Un archivo que no se puede leer o parsear no tumba el analisis del
			// resto: se cuenta como no resuelto, que es lo que es.
			structure.Unresolved = append(structure.Unresolved, file.Path)

			continue
		}

		structure.Functions = append(structure.Functions, attribution.Functions...)
		structure.OrphanStmts += attribution.OrphanStmts
	}

	return structure, nil
}
