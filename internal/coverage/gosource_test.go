package coverage

import (
	"os"
	"path/filepath"
	"testing"
)

// El go.mod se busca subiendo desde donde se pide, porque el analisis se lanza
// desde el directorio del proyecto y no necesariamente desde su raiz.
func TestFindModuleWalksUp(t *testing.T) {
	module, err := FindModule(filepath.Join("testdata", "gomod", "nested"))
	if err != nil {
		t.Fatalf("FindModule returned error: %v", err)
	}

	if module.Path != "example.com/app" {
		t.Errorf("expected module path example.com/app, got %q", module.Path)
	}
	expectedDir, err := filepath.Abs(filepath.Join("testdata", "gomod"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if module.Dir != expectedDir {
		t.Errorf("expected module dir %q, got %q", expectedDir, module.Dir)
	}
}

// Se acepta un archivo y no solo un directorio: quien llama suele tener a mano la
// ruta del reporte, no la del proyecto.
func TestFindModuleAcceptsAFilePath(t *testing.T) {
	module, err := FindModule(filepath.Join("testdata", "gomod", "go.mod"))
	if err != nil {
		t.Fatalf("FindModule returned error: %v", err)
	}
	if module.Path != "example.com/app" {
		t.Errorf("expected module path example.com/app, got %q", module.Path)
	}
}

// Sin go.mod el error tiene que nombrar donde se empezo a buscar, para no dejar
// al usuario adivinando desde que directorio corrio el comando.
func TestFindModuleFailsWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	if _, err := FindModule(dir); err == nil {
		t.Fatal("expected an error when there is no go.mod above the path")
	}
}

// La directiva admite comillas y comentarios al final, y las dos formas tienen
// que dar la misma ruta.
func TestModulePathHandlesQuotesAndComments(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gomod", "quoted.mod"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	path, err := modulePath(data)
	if err != nil {
		t.Fatalf("modulePath returned error: %v", err)
	}
	if path != "example.com/quoted" {
		t.Errorf("expected example.com/quoted, got %q", path)
	}
}

// `module` como prefijo de otra palabra no es la directiva.
func TestModulePathIgnoresLookalikeLines(t *testing.T) {
	if _, err := modulePath([]byte("moduleFoo = 1\n\ngo 1.25.0\n")); err == nil {
		t.Fatal("expected an error: there is no module directive here")
	}
}

// El perfil nombra los archivos por import path. Quitarle el prefijo del modulo es
// todo lo que hace falta para llegar al archivo en disco.
func TestSourcePathStripsTheModulePrefix(t *testing.T) {
	module := Module{Path: "github.com/devherd/devherd", Dir: filepath.FromSlash("/home/dev/devherd")}

	path, ok := module.SourcePath("github.com/devherd/devherd/internal/cli/root.go")
	if !ok {
		t.Fatal("expected the path to resolve")
	}
	expected := filepath.Join("/home/dev/devherd", "internal", "cli", "root.go")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// Un archivo de otro modulo no es un fallo del analisis: pasa al concatenar
// perfiles, y quien llama decide si lo reporta o lo ignora.
func TestSourcePathReportsForeignPathsWithoutFailing(t *testing.T) {
	module := Module{Path: "github.com/devherd/devherd", Dir: "/home/dev/devherd"}

	cases := []string{
		"github.com/other/lib/thing.go",
		// El propio nombre del modulo no es un archivo.
		"github.com/devherd/devherd",
		// Prefijo compartido pero modulo distinto: no debe colarse.
		"github.com/devherd/devherd-extra/thing.go",
		"",
	}

	for _, profilePath := range cases {
		t.Run(profilePath, func(t *testing.T) {
			if path, ok := module.SourcePath(profilePath); ok {
				t.Errorf("expected %q not to resolve, got %q", profilePath, path)
			}
		})
	}
}
