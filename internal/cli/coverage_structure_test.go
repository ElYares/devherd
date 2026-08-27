package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goModuleFixture arma un modulo minimo en disco: un go.mod, un fuente y el perfil
// que lo mide. Es lo que `--structure` necesita para traducir las rutas del
// perfil, que vienen como import paths, a archivos que pueda parsear.
func goModuleFixture(t *testing.T) (root string, profile string) {
	t.Helper()

	root = t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}

		return path
	}

	write("go.mod", "module example.com/fixture\n\ngo 1.25.0\n")
	write("cmd.go", `package fixture

type Command struct {
	Use  string
	RunE func() error
}

func newThingCmd() *Command {
	cmd := &Command{
		Use: "thing",
		RunE: func() error {
			a := 1
			b := 2
			c := a + b
			_ = c

			return nil
		},
	}

	return cmd
}

func helper(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
`)

	// El constructor y helper cubiertos; el RunE entero sin cubrir.
	profile = write("coverage.out", `mode: set
example.com/fixture/cmd.go:8.29,21.2 2 1
example.com/fixture/cmd.go:11.16,17.14 8 0
example.com/fixture/cmd.go:24.24,30.2 3 2
`)

	return root, profile
}

// Los tres numeros que convierten un porcentaje en una decision tienen que salir
// juntos: el suelto, el techo, y cuanto de lo alcanzable esta cubierto.
func TestCoverageStructureReportsTheCeiling(t *testing.T) {
	root, profile := goModuleFixture(t)

	out, err := runCoverageCmd(t, root, "--report", profile, "--structure")
	if err != nil {
		t.Fatalf("coverage --structure returned error: %v\n%s", err, out)
	}

	for _, expected := range []string{"covered", "reachable ceiling", "covered of what is reachable"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected the output to report %q, got:\n%s", expected, out)
		}
	}
	// 13 sentencias en total, 8 dentro del RunE guardado: techo 5/13 = 38,5%.
	if !strings.Contains(out, "38.5%") {
		t.Errorf("expected a ceiling of 38.5%%, got:\n%s", out)
	}
	// Las 5 alcanzables estan cubiertas del todo.
	if !strings.Contains(out, "100.0%") {
		t.Errorf("expected the reachable mass to be fully covered, got:\n%s", out)
	}
}

// La marca de "guardado" va en la fila de la funcion: es el dato que cambia que
// hacer con ella, y en una leyenda al pie no se lee.
func TestCoverageStructureMarksStoredClosuresInline(t *testing.T) {
	root, profile := goModuleFixture(t)

	out, err := runCoverageCmd(t, root, "--report", profile, "--structure")
	if err != nil {
		t.Fatalf("coverage --structure returned error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "newThingCmd.RunE") {
		t.Errorf("expected the closure to be named after the field it is assigned to, got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "newThingCmd.RunE") {
			continue
		}
		if !strings.Contains(line, "stored") {
			t.Errorf("expected the RunE row to be marked as stored, got: %q", line)
		}

		return
	}
}

// Para un formato sin rangos de linea se dice que solo hay agregacion por archivo.
// Inventar un techo seria peor que no ofrecer la funcion, y fallar con un error
// mandaria a buscar un problema que no existe.
func TestCoverageStructureIsHonestAboutOtherStacks(t *testing.T) {
	profile := writeCoverageFixture(t, "coverage.lcov", lcovTwoFiles)

	out, err := runCoverageCmd(t, "--report", profile, "--structure")
	if err != nil {
		t.Fatalf("expected --structure on lcov to explain itself, not fail: %v\n%s", err, out)
	}

	if !strings.Contains(out, "only available for Go") {
		t.Errorf("expected the output to say the analysis is Go-only, got:\n%s", out)
	}
	if !strings.Contains(out, "lcov") {
		t.Errorf("expected the output to name the format that was read, got:\n%s", out)
	}
	if !strings.Contains(out, "without --structure") {
		t.Errorf("expected the output to point at the per-file summary, got:\n%s", out)
	}
}

// El JSON estructural lleva la atribucion por funcion, que es lo que permite
// comparar dos corridas sin volver a mirar la tabla.
func TestCoverageStructureJSONCarriesFunctionAttribution(t *testing.T) {
	root, profile := goModuleFixture(t)

	out, err := runCoverageCmd(t, root, "--report", profile, "--structure", "--json")
	if err != nil {
		t.Fatalf("coverage --structure --json returned error: %v\n%s", err, out)
	}

	var payload struct {
		Format    string `json:"format"`
		Unit      string `json:"unit"`
		Functions []struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			Stored  bool   `json:"stored"`
			Total   int    `json:"total"`
			Covered int    `json:"covered"`
		} `json:"functions"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode structural JSON: %v\n%s", err, out)
	}

	if payload.Format != "go" || payload.Unit != "statements" {
		t.Errorf("expected go/statements in the payload, got %q/%q", payload.Format, payload.Unit)
	}
	if len(payload.Functions) != 3 {
		t.Fatalf("expected 3 attributed functions, got %d: %+v", len(payload.Functions), payload.Functions)
	}

	for _, fn := range payload.Functions {
		if fn.Name != "newThingCmd.RunE" {
			continue
		}
		if fn.Kind != "closure" || !fn.Stored || fn.Total != 8 || fn.Covered != 0 {
			t.Errorf("unexpected attribution for the RunE closure: %+v", fn)
		}

		return
	}
	t.Errorf("the RunE closure is missing from the payload: %+v", payload.Functions)
}

// Sin go.mod no hay como traducir las rutas del perfil. El comando tiene que
// decirlo, no devolver una tabla vacia que parezca un proyecto sin cobertura.
func TestCoverageStructureFailsWithoutAModule(t *testing.T) {
	_, profile := goModuleFixture(t)
	elsewhere := t.TempDir()

	out, err := runCoverageCmd(t, elsewhere, "--report", profile, "--structure")
	if err == nil {
		t.Fatalf("expected an error when the target has no go.mod, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("expected the error to name go.mod, got: %v", err)
	}
}
