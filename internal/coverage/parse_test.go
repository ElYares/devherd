package coverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}

	return data
}

func parseTestdata(t *testing.T, name string) Report {
	t.Helper()

	report, err := Parse(readTestdata(t, name))
	if err != nil {
		t.Fatalf("Parse(%s) returned error: %v", name, err)
	}

	return report
}

func fileByPath(t *testing.T, report Report, path string) FileReport {
	t.Helper()

	for _, file := range report.Files {
		if file.Path == path {
			return file
		}
	}

	t.Fatalf("file %q not found in report; files: %#v", path, report.Files)

	return FileReport{}
}

// Cada formato tiene que producir el mismo modelo, con su unidad correcta.
func TestParseDetectsEveryFormat(t *testing.T) {
	cases := []struct {
		file   string
		format string
		unit   Unit
		files  int
	}{
		{"sample.lcov", "lcov", UnitLines, 2},
		{"sample.gocover", "go", UnitStatements, 2},
		{"sample-clover.xml", "clover", UnitStatements, 2},
		{"sample-jacoco.xml", "jacoco", UnitLines, 2},
		{"sample-cobertura.xml", "cobertura", UnitLines, 1},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			report := parseTestdata(t, tc.file)
			if report.Format != tc.format {
				t.Fatalf("expected format %q, got %q", tc.format, report.Format)
			}
			if report.Unit != tc.unit {
				t.Fatalf("expected unit %q, got %q", tc.unit, report.Unit)
			}
			if len(report.Files) != tc.files {
				t.Fatalf("expected %d files, got %d: %#v", tc.files, len(report.Files), report.Files)
			}
		})
	}
}

// El formato se infiere del contenido, no de la extension: un jacoco renombrado
// se sigue detectando.
func TestParseIgnoresTheFileExtension(t *testing.T) {
	data := readTestdata(t, "sample-jacoco.xml")

	report, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if report.Format != "jacoco" {
		t.Fatalf("expected jacoco detected by structure, got %q", report.Format)
	}

	// Y escrito con otro nombre, el resultado es identico.
	path := filepath.Join(t.TempDir(), "cobertura.xml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write renamed report: %v", err)
	}
	renamed, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if renamed.Format != "jacoco" {
		t.Fatalf("a renamed jacoco report must still parse as jacoco, got %q", renamed.Format)
	}
}

// Clover y Cobertura comparten la raiz <coverage>; el desempate es el primer hijo.
func TestParseDistinguishesCloverFromCobertura(t *testing.T) {
	clover := parseTestdata(t, "sample-clover.xml")
	cobertura := parseTestdata(t, "sample-cobertura.xml")

	if clover.Format == cobertura.Format {
		t.Fatalf("clover and cobertura must not resolve to the same parser, both are %q", clover.Format)
	}
	if clover.Unit == cobertura.Unit {
		t.Fatalf("clover counts statements and cobertura lines; both reported %q", clover.Unit)
	}
}

func TestParseRejectsUnknownContentNamingTheFormatsItTried(t *testing.T) {
	_, err := Parse(readTestdata(t, "notacoverage.txt"))
	if err == nil {
		t.Fatal("expected an error for content that is not a coverage report")
	}

	for _, format := range SupportedFormats() {
		if !strings.Contains(err.Error(), format) {
			t.Fatalf("the error should name every format it tried; %q missing from %q", format, err)
		}
	}
}

func TestParseFileReportsAMissingFile(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "no-existe.out"))
	if err == nil {
		t.Fatal("expected an error for a missing report")
	}
	if !strings.Contains(err.Error(), "read coverage report") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Un reporte sin unidades medibles no es un 0%: es la ausencia del dato.
func TestParseEmptyReportIsEmptyNotZeroPercent(t *testing.T) {
	report := parseTestdata(t, "empty.gocover")

	if !report.IsEmpty() {
		t.Fatalf("expected an empty report, got %d units", report.Total())
	}
	if len(report.Files) != 0 {
		t.Fatalf("expected no files, got %#v", report.Files)
	}
}

// El perfil de Go trae varios bloques por archivo y hay que sumar sentencias, no
// contar lineas.
func TestGoProfileCountsStatements(t *testing.T) {
	report := parseTestdata(t, "sample.gocover")

	handler := fileByPath(t, report, "example.com/app/handler.go")
	if handler.Total != 5 || handler.Covered != 2 {
		t.Fatalf("expected 2/5 statements in handler.go, got %d/%d", handler.Covered, handler.Total)
	}

	if report.Total() != 6 || report.Covered() != 3 {
		t.Fatalf("expected 3/6 statements overall, got %d/%d", report.Covered(), report.Total())
	}
}

// Bloques repetidos se fusionan, no se suman: `go tool cover` hace lo mismo, y
// sumarlos inflaria el total. El fixture repite un bloque cubierto dos veces y uno
// **sin cubrir** tres veces: el segundo caso es el que fallaba, porque comprobar
// presencia contra el cero del mapa dejaba sin guardar los bloques con count 0.
func TestGoProfileMergesDuplicatedBlocks(t *testing.T) {
	report := parseTestdata(t, "duplicated.gocover")

	if report.Total() != 5 {
		t.Fatalf("expected 5 statements after merging duplicates, got %d", report.Total())
	}
	if report.Covered() != 2 {
		t.Fatalf("expected the duplicated covered block counted once, got %d", report.Covered())
	}
	if got := report.Percent(); got < 39.9 || got > 40.1 {
		t.Fatalf("expected 40.0%%, got %.1f%%", got)
	}
}

func TestLcovCountsLinesFromDAEntries(t *testing.T) {
	report := parseTestdata(t, "sample.lcov")

	format := fileByPath(t, report, "src/lib/format.ts")
	if format.Total != 4 || format.Covered != 2 {
		t.Fatalf("expected 2/4 lines in format.ts, got %d/%d", format.Covered, format.Total)
	}
}

// Algunas herramientas emiten solo el resumen LF/LH. Es menos preciso, pero
// descartarlo dejaria al usuario sin nada.
func TestLcovFallsBackToSummaryCounters(t *testing.T) {
	report := parseTestdata(t, "summary-only.lcov")

	legacy := fileByPath(t, report, "src/legacy.js")
	if legacy.Total != 10 || legacy.Covered != 3 {
		t.Fatalf("expected 3/10 lines from LF/LH, got %d/%d", legacy.Covered, legacy.Total)
	}
}

// PHPUnit emite lineas type="method" ademas de type="stmt". Contarlas inflaria el
// total y dejaria de coincidir con lo que el propio PHPUnit reporta.
func TestCloverCountsStatementLines(t *testing.T) {
	report := parseTestdata(t, "sample-clover.xml")

	controller := fileByPath(t, report, "/app/Http/Controllers/UserController.php")
	if controller.Total != 4 || controller.Covered != 3 {
		t.Fatalf("expected 3/4 statements ignoring method lines, got %d/%d", controller.Covered, controller.Total)
	}
	// El <metrics> del proyecto es un agregado y no debe contarse como archivo.
	if report.Total() != 6 {
		t.Fatalf("expected 6 statements total, got %d", report.Total())
	}
}

// JaCoCo trae varios contadores por archivo. Hay que tomar LINE: INSTRUCTION mide
// bytecode y no es comparable con lo que miden los otros formatos.
func TestJacocoUsesLineCountersAndQualifiesPaths(t *testing.T) {
	report := parseTestdata(t, "sample-jacoco.xml")

	service := fileByPath(t, report, "com/example/app/Service.java")
	if service.Total != 2 || service.Covered != 1 {
		t.Fatalf("expected 1/2 LINE units in Service.java, got %d/%d", service.Covered, service.Total)
	}
	// Con INSTRUCTION habrian salido 1000 unidades en vez de 2.
	if report.Total() != 12 {
		t.Fatalf("expected 12 line units overall, got %d", report.Total())
	}

	repository := fileByPath(t, report, "com/example/app/Repository.java")
	if repository.Total != 10 || repository.Covered != 2 {
		t.Fatalf("expected 2/10 lines in Repository.java, got %d/%d", repository.Covered, repository.Total)
	}
}

// coverage.py parte un archivo en varias <class>. Sumarlas contaria lineas
// repetidas; hay que fusionar por archivo y numero de linea.
func TestCoberturaMergesClassesOfTheSameFile(t *testing.T) {
	report := parseTestdata(t, "sample-cobertura.xml")

	views := fileByPath(t, report, "app/views.py")
	if views.Total != 3 || views.Covered != 2 {
		t.Fatalf("expected 2/3 lines after merging classes, got %d/%d", views.Covered, views.Total)
	}
}
