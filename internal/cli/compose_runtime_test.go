package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/detector"
	"github.com/devherd/devherd/internal/observe"
	"github.com/devherd/devherd/internal/proxy"
)

// newTestAppContext arma un appContext completo contra disco temporal y una base
// real migrada. No hace falta inyectar nada: prepareComposeProject y compania ya
// reciben el appContext como parametro; lo unico que faltaba era construirlo.
func newTestAppContext(t *testing.T, cfg config.Config) *appContext {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	manager := database.NewManager(filepath.Join(root, "devherd.db"))
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("Ensure database returned error: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open database returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return &appContext{
		Paths:  config.Paths{DataDir: root, ComposeDir: filepath.Join(root, "compose")},
		Config: cfg,
		DB:     db,
	}
}

// newComposeProjectDir crea un proyecto minimo en disco: solo un compose que
// ResolveProject pueda encontrar.
func newComposeProjectDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	compose := "services:\n  web:\n    image: nginx\n"
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	return root
}

// withProxyManifest le agrega al proyecto la metadata que el proxy externo exige.
// Sin ella BuildExternalProject falla, que es justo el caso del test del override
// de Observe perdido.
func withProxyManifest(t *testing.T, root string) string {
	t.Helper()

	manifest := "version: 1\ncompose:\n  files:\n    - docker-compose.yml\nproxy:\n  service: web\n  port: 80\n"
	if err := os.WriteFile(filepath.Join(root, ".devherd.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write devherd manifest: %v", err)
	}

	return root
}

func externalProxyConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Default()
	cfg.Proxy.Driver = proxy.DriverCaddyDockerExternal
	cfg.Proxy.ExternalDir = t.TempDir()
	cfg.Proxy.ExternalNetwork = "infra_web"

	return cfg
}

func TestAppendObserveOverrideOnlyWhenTheFileExists(t *testing.T) {
	root := newComposeProjectDir(t)

	project, err := compose.ResolveProject(root)
	if err != nil {
		t.Fatalf("ResolveProject returned error: %v", err)
	}
	before := len(project.ComposeFiles)

	if got := appendObserveOverride(project); len(got.ComposeFiles) != before {
		t.Fatalf("expected no extra compose file without the override, got %v", got.ComposeFiles)
	}

	overridePath := filepath.Join(root, observe.ManagedComposeOverrideFile)
	if err := os.WriteFile(overridePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write observe override: %v", err)
	}

	got := appendObserveOverride(project)
	if len(got.ComposeFiles) != before+1 {
		t.Fatalf("expected the observe override appended, got %v", got.ComposeFiles)
	}
	if got.ComposeFiles[len(got.ComposeFiles)-1] != overridePath {
		t.Fatalf("expected %q last, got %v", overridePath, got.ComposeFiles)
	}
}

// Sin proxy externo no hay override que generar: el proyecto sale como entro,
// salvo el de Observe.
func TestPrepareComposeProjectSkipsProxyOverrideWithoutExternalDriver(t *testing.T) {
	root := newComposeProjectDir(t)
	app := newTestAppContext(t, config.Default())

	project, err := prepareComposeProject(context.Background(), app, root)
	if err != nil {
		t.Fatalf("prepareComposeProject returned error: %v", err)
	}

	for _, file := range project.ComposeFiles {
		if strings.Contains(file, "caddy") || strings.Contains(filepath.Dir(file), app.Config.Proxy.ExternalDir) {
			t.Fatalf("unexpected proxy override with the default driver: %v", project.ComposeFiles)
		}
	}
}

// Con appContext nil el comando sigue funcionando en modo degradado. Es el camino
// que usan los comandos cuando DevHerd no esta inicializado.
func TestPrepareComposeProjectToleratesNilAppContext(t *testing.T) {
	root := newComposeProjectDir(t)

	project, err := prepareComposeProject(context.Background(), nil, root)
	if err != nil {
		t.Fatalf("prepareComposeProject returned error: %v", err)
	}
	if len(project.ComposeFiles) == 0 {
		t.Fatal("expected the project compose file to survive")
	}
}

func TestPrepareComposeProjectAddsProxyAndObserveOverrides(t *testing.T) {
	root := withProxyManifest(t, newComposeProjectDir(t))
	overridePath := filepath.Join(root, observe.ManagedComposeOverrideFile)
	if err := os.WriteFile(overridePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write observe override: %v", err)
	}

	cfg := externalProxyConfig(t)
	app := newTestAppContext(t, cfg)
	seedProject(t, app.DB, root)

	project, err := prepareComposeProject(context.Background(), app, root)
	if err != nil {
		t.Fatalf("prepareComposeProject returned error: %v", err)
	}

	if len(project.ComposeFiles) < 3 {
		t.Fatalf("expected compose + proxy override + observe override, got %v", project.ComposeFiles)
	}
	if project.ComposeFiles[len(project.ComposeFiles)-1] != overridePath {
		t.Fatalf("the observe override has to go last, got %v", project.ComposeFiles)
	}
}

// Un proyecto que no esta en la base no es un error: se detecta al vuelo y se
// arma el registro con lo que haya.
func TestResolveExternalProjectFallsBackToTheDetector(t *testing.T) {
	root := withProxyManifest(t, newComposeProjectDir(t))
	app := newTestAppContext(t, externalProxyConfig(t))

	external, err := resolveExternalProject(context.Background(), app, root)
	if err != nil {
		t.Fatalf("resolveExternalProject returned error: %v", err)
	}

	if external.Project.Name != filepath.Base(root) {
		t.Fatalf("expected the directory name as project name, got %q", external.Project.Name)
	}
}

func TestResolveExternalProjectUsesTheRegisteredRecord(t *testing.T) {
	root := withProxyManifest(t, newComposeProjectDir(t))
	app := newTestAppContext(t, externalProxyConfig(t))
	seedProject(t, app.DB, root)

	external, err := resolveExternalProject(context.Background(), app, root)
	if err != nil {
		t.Fatalf("resolveExternalProject returned error: %v", err)
	}

	if external.Project.Name != "registrado" {
		t.Fatalf("expected the name stored in the database, got %q", external.Project.Name)
	}
}

func seedProject(t *testing.T, db *sql.DB, root string) {
	t.Helper()

	err := database.UpsertProject(context.Background(), db, detector.Project{
		Name:      "registrado",
		Path:      root,
		Framework: "laravel",
		Stack:     "php",
		Runtime:   "docker",
	}, "registrado.test")
	if err != nil {
		t.Fatalf("UpsertProject returned error: %v", err)
	}
}

// Regresion: un proyecto en modo proxy externo cuyo target no se puede resolver
// perdia tambien el override de Observe, asi que se levantaba sin DSN y dejaba de
// reportar sin decir nada. Son dos overrides independientes.
func TestPrepareComposeProjectKeepsObserveOverrideWhenProxyCannotResolve(t *testing.T) {
	root := newComposeProjectDir(t) // sin manifiesto: el proxy no resuelve
	overridePath := filepath.Join(root, observe.ManagedComposeOverrideFile)
	if err := os.WriteFile(overridePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write observe override: %v", err)
	}

	app := newTestAppContext(t, externalProxyConfig(t))

	project, err := prepareComposeProject(context.Background(), app, root)
	if err != nil {
		t.Fatalf("prepareComposeProject returned error: %v", err)
	}

	for _, file := range project.ComposeFiles {
		if file == overridePath {
			return
		}
	}

	t.Fatalf("the observe override was dropped along with the proxy one: %v", project.ComposeFiles)
}
