package coverage

import (
	"errors"
	"path/filepath"
	"testing"
)

// cmdlikeStructure arma un reporte estructural a partir del fixture congelado,
// sin pasar por la resolucion de modulo, que tiene sus propias pruebas.
func cmdlikeStructure(t *testing.T) StructureReport {
	t.Helper()

	attribution := cmdlikeAttribution(t)

	return StructureReport{
		Format:      "go",
		Unit:        UnitStatements,
		Functions:   attribution.Functions,
		OrphanStmts: attribution.OrphanStmts,
	}
}

// El caso que da sentido al comando: el porcentaje suelto dice una cosa y el
// techo dice otra, y solo el segundo permite decidir si vale la pena insistir.
func TestCeilingSeparatesAbandonedCodeFromUnreachableCode(t *testing.T) {
	structure := cmdlikeStructure(t)

	if got := structure.Total(); got != 100 {
		t.Fatalf("expected 100 statements, got %d", got)
	}
	if got := structure.Percent(); got != 27 {
		t.Errorf("expected 27%% covered, got %.1f%%", got)
	}
	// 69 de las 100 sentencias viven en closures, pero solo 60 estan **guardadas**
	// en un literal de struct. Las otras 9 —una asignada a variable local, otra
	// pasada como argumento— se ejecutan cuando corre su funcion, y meterlas en el
	// techo lo inflaria con codigo que ya es probable hoy. La diferencia entre 69 y
	// 60 es exactamente lo que separa esta regla de "todo closure es inalcanzable".
	if got := structure.StoredTotal(); got != 60 {
		t.Errorf("expected 60 stored statements, got %d", got)
	}
	if got := structure.ReachableTotal(); got != 40 {
		t.Errorf("expected a reachable mass of 40, got %d", got)
	}
	if got := structure.Ceiling(); got != 40 {
		t.Errorf("expected a ceiling of 40%%, got %.1f%%", got)
	}
	// 27 de 40: el 27% suelto parecia abandono y es el 67,5% de lo alcanzable.
	if got := structure.PercentOfReachable(); got != 67.5 {
		t.Errorf("expected 67.5%% of the reachable mass, got %.1f%%", got)
	}
}

// Un closure asignado a una variable local **si** se ejecuta cuando corre su
// funcion. Contarlo como inalcanzable inflaria el techo con codigo que ya es
// probable escribiendo un test mas, sin tocar la estructura.
func TestOnlyClosuresStoredIntoStructuresLeaveTheCeiling(t *testing.T) {
	attribution := cmdlikeAttribution(t)

	cases := map[string]bool{
		// Guardado en el campo RunE de un literal: hay que armar el Command.
		"newThingCmd.RunE": false,
		// Asignado a una variable local y llamado ahi mismo.
		"withVar.double": true,
		// Pasado como argumento: lo ejecuta la llamada que lo recibe.
		"anonymous.func1": true,
		// Las declaraciones siempre son alcanzables.
		"helper":     true,
		"Store.Save": true,
	}

	for name, reachable := range cases {
		t.Run(name, func(t *testing.T) {
			fn := functionByName(t, attribution, name)
			if fn.Reachable() != reachable {
				t.Errorf("expected Reachable()=%v for %s, got %v", reachable, name, fn.Reachable())
			}
		})
	}
}

// Ordenar por masa sin cubrir y no por porcentaje: es la unica forma de que la
// primera fila sea donde conviene trabajar.
func TestStructureByUncoveredRanksByMassNotPercentage(t *testing.T) {
	structure := cmdlikeStructure(t)
	ranked := structure.ByUncovered()

	if len(ranked) == 0 {
		t.Fatal("expected functions in the ranking")
	}
	if ranked[0].Name != "newThingCmd.RunE" {
		t.Errorf("expected the 60-statement closure first, got %q with %d uncovered",
			ranked[0].Name, ranked[0].Uncovered())
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1].Uncovered() < ranked[i].Uncovered() {
			t.Fatalf("ranking is not sorted by uncovered mass at %d: %d then %d",
				i, ranked[i-1].Uncovered(), ranked[i].Uncovered())
		}
	}
}

// Para un formato sin rangos de linea hay que decir que solo hay agregacion por
// archivo, no devolver un techo inventado ni un reporte vacio que parezca 0%.
func TestStructureRefusesFormatsWithoutBlocks(t *testing.T) {
	for _, name := range []string{"sample.lcov", "sample-jacoco.xml", "sample-clover.xml", "sample-cobertura.xml"} {
		t.Run(name, func(t *testing.T) {
			report := parseTestdata(t, name)

			_, err := Structure(report, ".")
			var unsupported ErrNoStructuralAnalysis
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected ErrNoStructuralAnalysis, got %v", err)
			}
			if unsupported.Format != report.Format {
				t.Errorf("the error should name the format read, expected %q got %q",
					report.Format, unsupported.Format)
			}
		})
	}
}

// Un archivo cuya fuente no esta en disco se lista como no resuelto en vez de
// tumbar el analisis o desaparecer sin dejar rastro.
func TestStructureListsUnresolvedFilesInsteadOfHidingThem(t *testing.T) {
	report := Report{
		Format: "go",
		Unit:   UnitStatements,
		Files: []FileReport{{
			Path:    "example.com/other/thing.go",
			Total:   3,
			Covered: 0,
			Blocks:  []Block{{StartLine: 1, EndLine: 3, Stmts: 3, Count: 0}},
		}},
	}

	structure, err := Structure(report, filepath.Join("testdata", "gomod"))
	if err != nil {
		t.Fatalf("Structure returned error: %v", err)
	}
	if len(structure.Unresolved) != 1 || structure.Unresolved[0] != "example.com/other/thing.go" {
		t.Errorf("expected the foreign file to be listed as unresolved, got %#v", structure.Unresolved)
	}
	if !structure.IsEmpty() {
		t.Errorf("nothing should have been attributed, got %#v", structure.Functions)
	}
}

// Sin go.mod no hay como traducir las rutas del perfil, y eso es un error del
// analisis, no un reporte vacio.
func TestStructureFailsWithoutAModule(t *testing.T) {
	report := parseTestdata(t, "cmdlike.gocover")

	if _, err := Structure(report, t.TempDir()); err == nil {
		t.Fatal("expected an error when there is no go.mod above the root")
	}
}

// Los porcentajes de un reporte sin nada medido son 0 y no una division por cero.
func TestEmptyStructureReportDoesNotDivideByZero(t *testing.T) {
	var structure StructureReport

	if !structure.IsEmpty() {
		t.Fatal("a report with no functions should be empty")
	}
	for name, got := range map[string]float64{
		"Percent":            structure.Percent(),
		"Ceiling":            structure.Ceiling(),
		"PercentOfReachable": structure.PercentOfReachable(),
	} {
		if got != 0 {
			t.Errorf("%s should be 0 on an empty report, got %v", name, got)
		}
	}
}
