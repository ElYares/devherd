package coverage

import (
	"path/filepath"
	"testing"
)

// cmdlikeAttribution atribuye el fixture congelado. El perfil referencia numeros
// de linea de cmdlike.go.txt, asi que los dos se mueven juntos o ninguno.
func cmdlikeAttribution(t *testing.T) FileAttribution {
	t.Helper()

	report := parseTestdata(t, "cmdlike.gocover")
	file := fileByPath(t, report, "example.com/app/cmdlike.go")

	attribution, err := AttributeFile(file, filepath.Join("testdata", "cmdlike.go.txt"))
	if err != nil {
		t.Fatalf("AttributeFile returned error: %v", err)
	}

	return attribution
}

func functionByName(t *testing.T, attribution FileAttribution, name string) FunctionReport {
	t.Helper()

	for _, fn := range attribution.Functions {
		if fn.Name == name {
			return fn
		}
	}

	names := make([]string, 0, len(attribution.Functions))
	for _, fn := range attribution.Functions {
		names = append(names, fn.Name)
	}
	t.Fatalf("function %q not attributed; got %v", name, names)

	return FunctionReport{}
}

// Cada funcion recibe las sentencias de los bloques que caen dentro de su rango,
// con el nombre calificado que le corresponde.
func TestAttributeFileNamesAndCountsEveryFunction(t *testing.T) {
	attribution := cmdlikeAttribution(t)

	expected := []FunctionReport{
		{Name: "newThingCmd", Kind: KindFunc, Total: 2, Covered: 2},
		{Name: "newThingCmd.RunE", Kind: KindClosure, Enclosing: "newThingCmd", Total: 60, Covered: 0},
		{Name: "helper", Kind: KindFunc, Total: 9, Covered: 9},
		{Name: "Store.Save", Kind: KindMethod, Total: 12, Covered: 8},
		{Name: "withVar", Kind: KindFunc, Total: 4, Covered: 4},
		{Name: "withVar.double", Kind: KindClosure, Enclosing: "withVar", Total: 4, Covered: 0},
		{Name: "anonymous", Kind: KindFunc, Total: 4, Covered: 4},
		{Name: "anonymous.func1", Kind: KindClosure, Enclosing: "anonymous", Total: 5, Covered: 0},
	}

	if len(attribution.Functions) != len(expected) {
		names := make([]string, 0, len(attribution.Functions))
		for _, fn := range attribution.Functions {
			names = append(names, fn.Name)
		}
		t.Fatalf("expected %d attributed functions, got %d: %v", len(expected), len(attribution.Functions), names)
	}

	for _, want := range expected {
		got := functionByName(t, attribution, want.Name)
		if got.Kind != want.Kind {
			t.Errorf("%s: expected kind %q, got %q", want.Name, want.Kind, got.Kind)
		}
		if got.Enclosing != want.Enclosing {
			t.Errorf("%s: expected enclosing %q, got %q", want.Name, want.Enclosing, got.Enclosing)
		}
		if got.Total != want.Total || got.Covered != want.Covered {
			t.Errorf("%s: expected %d/%d covered/total, got %d/%d",
				want.Name, want.Covered, want.Total, got.Covered, got.Total)
		}
	}
}

// El caso que motiva la historia entera, con los valores medidos en `internal/cli`
// el 2026-08-24 reproducidos sobre un fixture que no se mueve: la mayor parte de
// la masa vive en closures, y el techo sin refactor es lo que queda fuera.
func TestAttributeFileReproducesTheClosureCeiling(t *testing.T) {
	attribution := cmdlikeAttribution(t)

	total, closures, covered := 0, 0, 0
	for _, fn := range attribution.Functions {
		total += fn.Total
		covered += fn.Covered
		if fn.Kind == KindClosure {
			closures += fn.Total
		}
	}

	if total != 100 {
		t.Fatalf("expected the fixture to measure 100 statements, got %d", total)
	}
	if closures != 69 {
		t.Errorf("expected 69%% of the mass inside closures, got %d%%", closures)
	}
	if ceiling := total - closures; ceiling != 31 {
		t.Errorf("expected a ceiling of 31%% without touching the constructors, got %d%%", ceiling)
	}
	if covered != 27 {
		t.Errorf("expected 27 covered statements, got %d", covered)
	}
}

// Un bloque pertenece a la funcion mas interna que lo contiene. Con la heuristica
// de "la ultima que empieza antes" que se uso a mano, los 60 statements del RunE
// se le habrian cargado al constructor, que es justo el error que la historia
// existe para no repetir.
func TestAttributeFileAssignsBlocksToTheInnermostFunction(t *testing.T) {
	attribution := cmdlikeAttribution(t)

	constructor := functionByName(t, attribution, "newThingCmd")
	if constructor.Total != 2 {
		t.Fatalf("the constructor should keep only its own 2 statements, got %d; "+
			"the closure's mass leaked into it", constructor.Total)
	}

	closure := functionByName(t, attribution, "newThingCmd.RunE")
	if closure.Enclosing != "newThingCmd" {
		t.Errorf("expected the closure to point back at its constructor, got %q", closure.Enclosing)
	}
}

// La atribucion no puede contradecir al reporte por archivo: es el mismo dato con
// mas detalle. Si difieren, la tabla estructural miente sobre un total que el
// resumen ya mostro.
func TestAttributeFileAddsUpToTheFileTotals(t *testing.T) {
	report := parseTestdata(t, "cmdlike.gocover")
	file := fileByPath(t, report, "example.com/app/cmdlike.go")
	attribution := cmdlikeAttribution(t)

	total, covered := attribution.OrphanStmts, attribution.OrphanCovered
	for _, fn := range attribution.Functions {
		total += fn.Total
		covered += fn.Covered
	}

	if total != file.Total {
		t.Errorf("attributed %d statements, the file reports %d", total, file.Total)
	}
	if covered != file.Covered {
		t.Errorf("attributed %d covered, the file reports %d", covered, file.Covered)
	}
	if attribution.OrphanStmts != 0 {
		t.Errorf("no block should fall outside every function, got %d orphan statements",
			attribution.OrphanStmts)
	}
}

// Un bloque que no cae en ninguna funcion se cuenta aparte, no se reparte a la
// fuerza. Pasa cuando el perfil y el fuente se desincronizan, y silenciarlo daria
// una atribucion torcida que nadie puede detectar.
func TestAttributeFileCountsOrphanBlocksInsteadOfForcingThem(t *testing.T) {
	file := FileReport{
		Path:    "example.com/app/cmdlike.go",
		Total:   7,
		Covered: 2,
		Blocks: []Block{
			{StartLine: 44, EndLine: 50, Stmts: 2, Count: 1},
			// Mas alla del final del archivo: ninguna funcion lo contiene.
			{StartLine: 900, EndLine: 910, Stmts: 5, Count: 0},
		},
	}

	attribution, err := AttributeFile(file, filepath.Join("testdata", "cmdlike.go.txt"))
	if err != nil {
		t.Fatalf("AttributeFile returned error: %v", err)
	}

	if attribution.OrphanStmts != 5 {
		t.Errorf("expected 5 orphan statements, got %d", attribution.OrphanStmts)
	}
	if len(attribution.Functions) != 1 || attribution.Functions[0].Name != "helper" {
		t.Errorf("expected only helper to be attributed, got %#v", attribution.Functions)
	}
}

// Las funciones que el perfil no menciona no se listan al 0%: no son abandono
// medido, son codigo que no se compilo en esa corrida.
func TestAttributeFileSkipsFunctionsWithoutMeasuredStatements(t *testing.T) {
	attribution := cmdlikeAttribution(t)

	for _, fn := range attribution.Functions {
		if fn.Total == 0 {
			t.Errorf("function %q was listed with no measured statements", fn.Name)
		}
	}
	for _, name := range []string{"simpleError.Error", "run", "noop"} {
		for _, fn := range attribution.Functions {
			if fn.Name == name {
				t.Errorf("%q has no blocks in the profile and should not be listed", name)
			}
		}
	}
}

// Un fuente que no existe o que no compila tiene que fallar con la ruta en el
// mensaje, no devolver una atribucion vacia que parezca un archivo sin cobertura.
func TestAttributeFileFailsLoudlyOnUnreadableSource(t *testing.T) {
	file := FileReport{Path: "example.com/app/gone.go", Total: 1, Blocks: []Block{{StartLine: 1, EndLine: 2, Stmts: 1}}}

	if _, err := AttributeFile(file, filepath.Join("testdata", "does-not-exist.go")); err == nil {
		t.Fatal("expected an error for a missing source file")
	}
	if _, err := AttributeFile(file, filepath.Join("testdata", "notacoverage.txt")); err == nil {
		t.Fatal("expected an error for a file that is not valid Go")
	}
}
