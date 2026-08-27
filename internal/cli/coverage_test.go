package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/coverage"
)

func runCoverageCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newCoverageCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return out.String(), err
}

// countListedFiles cuenta solo las lineas de archivo. La palabra "uncovered"
// tambien aparece en el encabezado y en la nota de omitidos, asi que contarla a
// secas da de mas; las lineas de archivo son las unicas que llevan porcentaje.
func countListedFiles(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "uncovered") && strings.Contains(line, "%") {
			count++
		}
	}

	return count
}

func writeCoverageFixture(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return path
}

const lcovTwoFiles = `TN:
SF:src/covered.ts
DA:1,1
DA:2,1
end_of_record
TN:
SF:src/half.ts
DA:1,1
DA:2,0
DA:3,0
DA:4,0
end_of_record
`

// La unidad va siempre en la cabecera: sin ella nadie sabe si el 50% son lineas
// o sentencias, y los dos numeros no son comparables.
func TestCoverageOutputAlwaysNamesTheUnit(t *testing.T) {
	path := writeCoverageFixture(t, "lcov.info", lcovTwoFiles)

	out, err := runCoverageCmd(t, "--report", path)
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "lines") {
		t.Fatalf("expected the unit in the header:\n%s", out)
	}
	if !strings.Contains(out, "lcov") {
		t.Fatalf("expected the detected format in the header:\n%s", out)
	}
}

// Dos archivos, uno al 100% y otro al 25%: el total es 3/6 = 50%, no el promedio
// de 100% y 25%, que daria 62,5%.
func TestCoverageTotalIsWeightedNotAveraged(t *testing.T) {
	path := writeCoverageFixture(t, "lcov.info", lcovTwoFiles)

	out, err := runCoverageCmd(t, "--report", path, "--all")
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "50.0%") {
		t.Fatalf("expected a weighted 50.0%% total:\n%s", out)
	}
	if strings.Contains(out, "62.5%") {
		t.Fatalf("the total must not be the average of per-file percentages:\n%s", out)
	}
	for _, want := range []string{"src/covered.ts", "src/half.ts"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q listed with --all:\n%s", want, out)
		}
	}
}

// Con 30 archivos se muestran 10 y se dice cuantos quedaron fuera. Truncar en
// silencio hace creer que se vio todo.
func TestCoverageLimitsTheListAndSaysWhatItOmitted(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < 30; i++ {
		builder.WriteString("TN:\n")
		fmt.Fprintf(&builder, "SF:src/file%02d.ts\n", i)
		builder.WriteString("DA:1,0\nDA:2,0\nend_of_record\n")
	}
	path := writeCoverageFixture(t, "lcov.info", builder.String())

	out, err := runCoverageCmd(t, "--report", path)
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}

	listed := countListedFiles(out)
	if listed != 10 {
		t.Fatalf("expected 10 files listed by default, got %d:\n%s", listed, out)
	}
	if !strings.Contains(out, "20 more file(s)") {
		t.Fatalf("expected the omitted count to be stated:\n%s", out)
	}

	full, err := runCoverageCmd(t, "--report", path, "--all")
	if err != nil {
		t.Fatalf("coverage --all returned error: %v\n%s", err, full)
	}
	if countListedFiles(full) != 30 {
		t.Fatalf("expected every file with --all:\n%s", full)
	}
}

func TestCoverageTopFlagChangesHowManyAreListed(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < 8; i++ {
		builder.WriteString("TN:\n")
		fmt.Fprintf(&builder, "SF:src/file%02d.ts\n", i)
		builder.WriteString("DA:1,0\nend_of_record\n")
	}
	path := writeCoverageFixture(t, "lcov.info", builder.String())

	out, err := runCoverageCmd(t, "--report", path, "--top", "3")
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	if got := countListedFiles(out); got != 3 {
		t.Fatalf("expected 3 files with --top 3, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "5 more file(s)") {
		t.Fatalf("expected the remainder stated:\n%s", out)
	}
}

func TestCoverageJSONSerializesTheReport(t *testing.T) {
	path := writeCoverageFixture(t, "lcov.info", lcovTwoFiles)

	out, err := runCoverageCmd(t, "--report", path, "--json")
	if err != nil {
		t.Fatalf("coverage --json returned error: %v\n%s", err, out)
	}

	var report coverage.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the JSON output must be parseable: %v\n%s", err, out)
	}
	if report.Format != "lcov" || report.Unit != coverage.UnitLines {
		t.Fatalf("unexpected JSON report: %#v", report)
	}
	if len(report.Files) != 2 || report.Total() != 6 || report.Covered() != 3 {
		t.Fatalf("unexpected JSON totals: %d/%d over %d files", report.Covered(), report.Total(), len(report.Files))
	}
}

// Un reporte sin unidades no es 0%: es la ausencia del dato, y confundirlos hace
// creer que no hay nada probado.
func TestCoverageSaysThereIsNoDataInsteadOfZeroPercent(t *testing.T) {
	path := writeCoverageFixture(t, "coverage.out", "mode: set\n")

	out, err := runCoverageCmd(t, "--report", path)
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no coverage data") {
		t.Fatalf("expected an explicit no-data message:\n%s", out)
	}
	if strings.Contains(out, "0.0%") {
		t.Fatalf("an empty report must not be shown as 0.0%%:\n%s", out)
	}
}

func TestCoverageFailsOnUnknownFormatWithoutPanicking(t *testing.T) {
	path := writeCoverageFixture(t, "coverage.out", "esto no es un reporte\n")

	out, err := runCoverageCmd(t, "--report", path)
	if err == nil {
		t.Fatalf("expected an error for an unrecognized report:\n%s", out)
	}
	for _, format := range coverage.SupportedFormats() {
		if !strings.Contains(err.Error(), format) {
			t.Fatalf("the error should name the formats tried; %q missing from %q", format, err)
		}
	}
}

func TestCoverageFailsOnMissingFile(t *testing.T) {
	_, err := runCoverageCmd(t, "--report", filepath.Join(t.TempDir(), "ausente.out"))
	if err == nil {
		t.Fatal("expected an error for a missing report")
	}
}

// `--report` dejo de ser obligatorio al llegar el autodescubrimiento. Lo que si
// sigue siendo obligatorio es que, sin nada que leer, el comando diga donde busco
// y como generar un reporte, en vez de pedir una bandera y ya.
func TestCoverageWithoutAnyReportSaysWhereItLooked(t *testing.T) {
	empty := t.TempDir()

	_, err := runCoverageCmd(t, empty)
	if err == nil {
		t.Fatal("expected an error when there is no report to read")
	}

	message := err.Error()
	for _, expected := range []string{"looked for", "coverage.out", "coverage/lcov.info"} {
		if !strings.Contains(message, expected) {
			t.Errorf("expected the error to contain %q, got:\n%s", expected, message)
		}
	}
}

// Con varios directorios se muestra el desglose; con uno solo seria ruido.
func TestCoverageGroupsByDirectoryWhenThereIsMoreThanOne(t *testing.T) {
	fixture := `TN:
SF:internal/cli/up.go
DA:1,0
DA:2,0
end_of_record
TN:
SF:internal/observe/store.go
DA:1,1
DA:2,1
end_of_record
`
	path := writeCoverageFixture(t, "lcov.info", fixture)

	out, err := runCoverageCmd(t, "--report", path)
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	for _, want := range []string{"directory", "internal/cli", "internal/observe"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in the grouped output:\n%s", want, out)
		}
	}
}

// Criterio 2 de HU-004: el total de un perfil de Go tiene que coincidir con lo
// que reporta `go tool cover -func` sobre el mismo archivo. Es la unica forma de
// saber que el parser no cuenta de mas ni de menos.
func TestCoverageMatchesGoToolCoverOnARealProfile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	build := exec.Command("go", "test", "./internal/coverage/", "-coverprofile="+profile)
	build.Dir = repoRootForTest(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("could not generate a coverage profile: %v\n%s", err, output)
	}

	funcOutput, err := exec.Command("go", "tool", "cover", "-func="+profile).CombinedOutput()
	if err != nil {
		t.Skipf("go tool cover unavailable: %v\n%s", err, funcOutput)
	}
	want := totalFromGoToolCover(t, string(funcOutput))

	report, err := coverage.ParseFile(profile)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	// Sin esto la prueba pasaria trivialmente con un perfil vacio: los dos lados
	// darian 0% y no se estaria comparando nada.
	if report.Total() == 0 {
		t.Fatal("the generated profile has no statements; the comparison would be vacuous")
	}

	got := report.Percent()
	if diff := got - want; diff > 0.05 || diff < -0.05 {
		t.Fatalf("parser reports %.1f%% but `go tool cover -func` reports %.1f%%", got, want)
	}
}

func totalFromGoToolCover(t *testing.T, output string) float64 {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "total:" {
			continue
		}

		value, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil {
			t.Fatalf("parse go tool cover total: %v", err)
		}

		return value
	}

	t.Fatalf("no total line in go tool cover output:\n%s", output)

	return 0
}

// repoRootForTest sube desde internal/cli hasta la raiz del modulo.
func repoRootForTest(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find the module root")

	return ""
}

// countListedDirs cuenta las filas de la tabla de directorios: llevan porcentaje y
// un par cubierto/total, pero no la palabra "uncovered" de la lista de archivos.
func countListedDirs(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "%") && strings.Contains(line, "/") &&
			!strings.Contains(line, "uncovered") && !strings.Contains(line, "total") {
			count++
		}
	}

	return count
}

// La tabla de directorios se acota igual que la de archivos. Dejarla completa
// volcaba 38 filas en un proyecto real mientras los archivos se limitaban a 10.
func TestCoverageLimitsTheDirectoryTable(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < 15; i++ {
		builder.WriteString("TN:\n")
		fmt.Fprintf(&builder, "SF:src/dir%02d/file.ts\n", i)
		builder.WriteString("DA:1,0\nDA:2,1\nend_of_record\n")
	}
	path := writeCoverageFixture(t, "lcov.info", builder.String())

	out, err := runCoverageCmd(t, "--report", path, "--top", "4")
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}
	if got := countListedDirs(out); got != 4 {
		t.Fatalf("expected 4 directories with --top 4, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "11 more directories") {
		t.Fatalf("expected the omitted directories stated:\n%s", out)
	}

	full, err := runCoverageCmd(t, "--report", path, "--all")
	if err != nil {
		t.Fatalf("coverage --all returned error: %v\n%s", err, full)
	}
	if got := countListedDirs(full); got != 15 {
		t.Fatalf("expected every directory with --all, got %d:\n%s", got, full)
	}
	if strings.Contains(full, "more directories") {
		t.Fatalf("--all must not report omitted directories:\n%s", full)
	}
}

// El bulto arriba: es lo que evita leer la tabla entera para encontrar el trabajo.
func TestCoverageDirectoryTableRanksByUncoveredMass(t *testing.T) {
	// Los nombres estan elegidos para que el orden alfabetico sea el **contrario**
	// al de masa: si la prueba pasara por orden alfabetico, no probaria nada.
	fixture := `TN:
SF:app/Aaa/cubierto.php
DA:1,1
DA:2,1
end_of_record
TN:
SF:app/Zzz/sin_cubrir.php
DA:1,0
DA:2,0
DA:3,0
DA:4,0
end_of_record
`
	path := writeCoverageFixture(t, "lcov.info", fixture)

	out, err := runCoverageCmd(t, "--report", path)
	if err != nil {
		t.Fatalf("coverage returned error: %v\n%s", err, out)
	}

	bulto := strings.Index(out, "app/Zzz")
	cubierto := strings.Index(out, "app/Aaa")
	if bulto < 0 || cubierto < 0 {
		t.Fatalf("expected both directories listed:\n%s", out)
	}
	if bulto > cubierto {
		t.Fatalf("expected app/Zzz first for having more uncovered mass:\n%s", out)
	}
}
