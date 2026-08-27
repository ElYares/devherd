package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El .env es lo unico que Jupyter necesita del entorno: que montar, con que uid y
// con que token. Va ahi y no en el compose porque el compose es de DevHerd y se
// regenera siempre.
func TestJupyterEnvCarriesWorkspaceAndToken(t *testing.T) {
	files, err := ServiceFiles("jupyter", ServiceOptions{
		Workspace: "/home/dev/develop",
		UID:       1000,
		GID:       1000,
	})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}
	if len(files) != 1 || files[0].Path != ".env" {
		t.Fatalf("expected a single .env, got %#v", files)
	}

	content := files[0].Content
	for _, want := range []string{
		"DEVHERD_WORKSPACE=/home/dev/develop",
		"DEVHERD_UID=1000",
		"DEVHERD_GID=1000",
		"JUPYTER_TOKEN=",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the env file should contain %q, got:\n%s", want, content)
		}
	}
}

// Sin directorio no se escribe una configuracion rota: un Jupyter que monta la
// nada arranca y no sirve, que es peor que uno que no arranca.
func TestJupyterRequiresAWorkspace(t *testing.T) {
	if _, err := ServiceFiles("jupyter", ServiceOptions{Workspace: "  "}); err == nil {
		t.Error("expected an error without a workspace")
	}
}

// **El token no es opcional.** Un Jupyter sin token es ejecucion de codigo
// arbitrario con escritura sobre todo lo montado, y aqui esta montado el codigo
// entero del usuario. Es la diferencia con Grafana, que solo lee y muestra.
func TestJupyterAlwaysCarriesARandomToken(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		files, err := ServiceFiles("jupyter", ServiceOptions{Workspace: "/tmp", UID: 1000, GID: 1000})
		if err != nil {
			t.Fatalf("ServiceFiles returned error: %v", err)
		}

		token := ""
		for _, line := range strings.Split(files[0].Content, "\n") {
			if strings.HasPrefix(line, "JUPYTER_TOKEN=") {
				token = strings.TrimPrefix(line, "JUPYTER_TOKEN=")
			}
		}
		if len(token) < 32 {
			t.Fatalf("the token is too short to be worth anything: %q", token)
		}
		if seen[token] {
			t.Fatal("the same token came out twice; it is not random")
		}
		seen[token] = true
	}
}

// El .env se genera, asi que no se puede comparar contra una plantilla: sale
// distinto cada vez. Sin GeneratedOnce, cada arranque lo veria como "editado por
// el usuario" y soltaria un aviso que no significa nada.
func TestJupyterEnvIsKeptSilentlyOnEveryRestart(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", "", "", "", ""}})
	ctx := context.Background()
	opts := StartOptions{Workspace: "/home/dev/develop", UID: 1000, GID: 1000}

	if _, files, err := m.Start(ctx, "jupyter", opts); err != nil {
		t.Fatalf("first Start returned error: %v", err)
	} else if files[0].State != FileWritten {
		t.Fatalf("expected the env to be written, got %v", files[0].State)
	}

	before, err := os.ReadFile(filepath.Join(m.dir, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}

	_, files, err := m.Start(ctx, "jupyter", opts)
	if err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if files[0].State != FileUnchanged {
		t.Errorf("a generated file that already exists must be silent, got %v", files[0].State)
	}
	if notice := DescribeFileResults(files); notice != "" {
		t.Errorf("expected no notice on restart, got %q", notice)
	}

	after, err := os.ReadFile(filepath.Join(m.dir, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	// El token tiene que sobrevivir: regenerarlo invalidaria la pestana abierta.
	if string(before) != string(after) {
		t.Error("the token was regenerated, invalidating any open session")
	}
}

// La URL sale con el token puesto. Mandar al usuario a buscarlo en los logs de un
// contenedor es la friccion que este comando existe para quitar.
func TestJupyterAccessURLCarriesTheToken(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", ""}})

	if _, _, err := m.Start(context.Background(), "jupyter", StartOptions{
		Workspace: "/home/dev/develop", UID: 1000, GID: 1000,
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	url, err := m.AccessURL("jupyter")
	if err != nil {
		t.Fatalf("AccessURL returned error: %v", err)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:8888/lab?token=") {
		t.Errorf("unexpected URL %q", url)
	}
	if len(strings.TrimPrefix(url, "http://127.0.0.1:8888/lab?token=")) < 32 {
		t.Errorf("the URL carries no usable token: %q", url)
	}
}

// El .env admite comentarios, y una clave mencionada en un comentario no es su
// valor. Lo aprendi probando a mano: un grep ingenuo saca el comentario.
func TestEnvValueSkipsComments(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	env := "# JUPYTER_TOKEN protege el servidor. No lo quites.\nJUPYTER_TOKEN=abc123\n"
	if err := os.WriteFile(filepath.Join(m.dir, ".env"), []byte(env), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	value, err := m.envValue("JUPYTER_TOKEN")
	if err != nil {
		t.Fatalf("envValue returned error: %v", err)
	}
	if value != "abc123" {
		t.Errorf("expected the value and not the comment, got %q", value)
	}
}

// El directorio por defecto es ~/develop cuando existe: es donde suele vivir el
// arbol de proyectos. Montar la raiz del sistema seria peor que preguntar.
func TestDefaultWorkspacePrefersTheDevelopTree(t *testing.T) {
	home := t.TempDir()
	if got := DefaultWorkspace(home); got != home {
		t.Errorf("without ~/develop the home is the workspace, got %q", got)
	}

	develop := filepath.Join(home, "develop")
	if err := os.Mkdir(develop, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := DefaultWorkspace(home); got != develop {
		t.Errorf("expected %q, got %q", develop, got)
	}
}

// Jupyter escucha solo en loopback, como el resto. Un Jupyter con token pero
// expuesto a la red sigue siendo una superficie que nadie pidio.
func TestJupyterIsPublishedOnLoopbackOnly(t *testing.T) {
	if !strings.Contains(composeContent, `"127.0.0.1:8888:8888"`) {
		t.Error("jupyter should publish its port on 127.0.0.1 only")
	}
}

// Jupyter entra al catalogo sin desplazar a nadie.
func TestJupyterIsPartOfTheCatalog(t *testing.T) {
	supported := SupportedServices()
	for _, want := range []string{"redis", "mailpit", "prometheus", "grafana", "jupyter"} {
		if !contains(supported, want) {
			t.Errorf("%s should be a supported service, got %v", want, supported)
		}
	}
	if !NeedsWorkspace("jupyter") {
		t.Error("jupyter needs a workspace")
	}
	for _, service := range []string{"redis", "mailpit", "prometheus", "grafana"} {
		if NeedsWorkspace(service) {
			t.Errorf("%s does not need a workspace", service)
		}
	}
}
