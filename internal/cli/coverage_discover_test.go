package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/coverage"
)

// projectWithReports arma un proyecto en disco con los reportes indicados.
func projectWithReports(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return root
}

const goProfileTwoFiles = `mode: set
example.com/app/a.go:1.1,3.2 2 1
example.com/app/b.go:1.1,4.2 3 0
`

// El caso que motiva la historia: `devherd coverage` a secas, sin recordar donde
// deja el reporte cada herramienta.
func TestCoverageDiscoversTheReportWithoutFlags(t *testing.T) {
	root := projectWithReports(t, map[string]string{
		"go.mod":       "module example.com/app\n\ngo 1.25.0\n",
		"coverage.out": goProfileTwoFiles,
	})

	out, err := runCoverageCmd(t, root)
	if err != nil {
		t.Fatalf("coverage without flags returned error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "using coverage.out") {
		t.Errorf("expected the output to name the report it used, got:\n%s", out)
	}
	if !strings.Contains(out, "go") {
		t.Errorf("expected the format to be reported, got:\n%s", out)
	}
}

// Con dos reportes gana el del stack detectado, y el otro se nombra. Tomar uno en
// silencio es como se lee el reporte del front creyendo que es el del back.
func TestCoverageDiscoveryMentionsTheReportItDidNotUse(t *testing.T) {
	root := projectWithReports(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25.0\n",
		"coverage.out":       goProfileTwoFiles,
		"coverage/lcov.info": lcovTwoFiles,
	})

	out, err := runCoverageCmd(t, root)
	if err != nil {
		t.Fatalf("coverage without flags returned error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "using coverage.out") {
		t.Errorf("expected the Go profile to win in a Go project, got:\n%s", out)
	}
	if !strings.Contains(out, "also found, not used") || !strings.Contains(out, "coverage/lcov.info") {
		t.Errorf("expected the lcov report to be mentioned, got:\n%s", out)
	}
}

// El reporte que deja --run se usa si es lo unico que hay, pero se dice que es
// suyo: puede ser de una corrida vieja.
func TestCoverageDiscoveryMarksTheManagedReport(t *testing.T) {
	root := projectWithReports(t, map[string]string{
		"go.mod":                              "module example.com/app\n\ngo 1.25.0\n",
		coverage.ManagedReportPrefix + ".out": goProfileTwoFiles,
	})

	out, err := runCoverageCmd(t, root)
	if err != nil {
		t.Fatalf("coverage without flags returned error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "devherd coverage --run") {
		t.Errorf("expected the output to say the report came from --run, got:\n%s", out)
	}
}

// `--report` explicito manda: no busca nada y no anuncia descubrimiento alguno.
func TestCoverageExplicitReportSkipsDiscovery(t *testing.T) {
	root := projectWithReports(t, map[string]string{
		"go.mod":       "module example.com/app\n\ngo 1.25.0\n",
		"coverage.out": goProfileTwoFiles,
	})
	elsewhere := writeCoverageFixture(t, "other.lcov", lcovTwoFiles)

	out, err := runCoverageCmd(t, root, "--report", elsewhere)
	if err != nil {
		t.Fatalf("coverage --report returned error: %v\n%s", err, out)
	}

	if strings.Contains(out, "using coverage.out") || strings.Contains(out, "also found") {
		t.Errorf("--report should skip discovery entirely, got:\n%s", out)
	}
	if !strings.Contains(out, "lcov") {
		t.Errorf("expected the explicit report to be the one read, got:\n%s", out)
	}
}

// El descubrimiento y --structure se combinan: se encuentra el perfil y se
// analiza sin pasar una sola ruta.
func TestCoverageDiscoveryFeedsTheStructuralAnalysis(t *testing.T) {
	root := projectWithReports(t, map[string]string{
		"go.mod":       "module example.com/app\n\ngo 1.25.0\n",
		"a.go":         "package app\n\nfunc A() int {\n\treturn 1\n}\n",
		"coverage.out": "mode: set\nexample.com/app/a.go:3.14,5.2 1 1\n",
	})

	out, err := runCoverageCmd(t, root, "--structure")
	if err != nil {
		t.Fatalf("coverage --structure returned error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "using coverage.out") {
		t.Errorf("expected discovery to run before the analysis, got:\n%s", out)
	}
	if !strings.Contains(out, "reachable ceiling") {
		t.Errorf("expected the structural report, got:\n%s", out)
	}
}
