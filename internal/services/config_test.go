package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withServiceFiles declara configuracion para un servicio durante una prueba.
// Hoy ningun servicio real declara ninguna —redis y mailpit no la necesitan, y
// por eso el problema no se veia— asi que la maquinaria se ejerce con uno
// ficticio hasta que llegue Prometheus.
func withServiceFiles(t *testing.T, service string, files ...ManagedFile) {
	t.Helper()

	previous, had := serviceFiles[service]
	serviceFiles[service] = files
	t.Cleanup(func() {
		if had {
			serviceFiles[service] = previous

			return
		}
		delete(serviceFiles, service)
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

// La primera vez, el archivo no existe y se escribe.
func TestEnsureFileWritesTheTemplateWhenAbsent(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	result, err := m.ensureFile(ManagedFile{Path: "conf/demo.yml", Content: "a: 1\n"}, false)
	if err != nil {
		t.Fatalf("ensureFile returned error: %v", err)
	}
	if result.State != FileWritten {
		t.Errorf("expected FileWritten, got %v", result.State)
	}
	if got := readFile(t, filepath.Join(m.dir, "conf", "demo.yml")); got != "a: 1\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

// **El criterio que da sentido a la historia.** Un archivo que el usuario edito
// sobrevive, y se dice que difiere de la plantilla.
func TestEnsureFileKeepsTheUserEdit(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})
	file := ManagedFile{Path: "demo.yml", Content: "a: 1\n"}
	path := filepath.Join(m.dir, "demo.yml")

	if _, err := m.ensureFile(file, false); err != nil {
		t.Fatalf("first ensureFile returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("a: 999  # lo mio\n"), 0o644); err != nil {
		t.Fatalf("simulate a user edit: %v", err)
	}

	result, err := m.ensureFile(file, false)
	if err != nil {
		t.Fatalf("second ensureFile returned error: %v", err)
	}
	if result.State != FileKept {
		t.Fatalf("expected the edit to be kept, got %v", result.State)
	}
	if got := readFile(t, path); got != "a: 999  # lo mio\n" {
		t.Errorf("the user edit was overwritten: %q", got)
	}

	notice := DescribeFileResults([]FileResult{result})
	if !strings.Contains(notice, "differs from the DevHerd template") {
		t.Errorf("the notice should say the file differs, got %q", notice)
	}
	if !strings.Contains(notice, "--force") {
		t.Errorf("the notice should point at --force, got %q", notice)
	}
}

// Un archivo identico a la plantilla no genera aviso: un aviso que sale siempre
// se aprende a ignorar.
func TestEnsureFileIsSilentWhenNothingChanged(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})
	file := ManagedFile{Path: "demo.yml", Content: "a: 1\n"}

	if _, err := m.ensureFile(file, false); err != nil {
		t.Fatalf("first ensureFile returned error: %v", err)
	}
	result, err := m.ensureFile(file, false)
	if err != nil {
		t.Fatalf("second ensureFile returned error: %v", err)
	}
	if result.State != FileUnchanged {
		t.Errorf("expected FileUnchanged, got %v", result.State)
	}
	if notice := DescribeFileResults([]FileResult{result}); notice != "" {
		t.Errorf("an unchanged file should produce no notice, got %q", notice)
	}
}

// --force restaura la plantilla, pero guarda una copia antes. Restaurar sin copia
// convierte una bandera en una perdida de trabajo.
func TestEnsureFileForceRestoresAndBacksUp(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})
	file := ManagedFile{Path: "demo.yml", Content: "a: 1\n"}
	path := filepath.Join(m.dir, "demo.yml")

	if _, err := m.ensureFile(file, false); err != nil {
		t.Fatalf("first ensureFile returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("a: 999\n"), 0o644); err != nil {
		t.Fatalf("simulate a user edit: %v", err)
	}

	result, err := m.ensureFile(file, true)
	if err != nil {
		t.Fatalf("forced ensureFile returned error: %v", err)
	}
	if result.State != FileRestored {
		t.Fatalf("expected FileRestored, got %v", result.State)
	}
	if got := readFile(t, path); got != "a: 1\n" {
		t.Errorf("the template was not restored: %q", got)
	}
	if got := readFile(t, result.Backup); got != "a: 999\n" {
		t.Errorf("the backup does not hold the user version: %q", got)
	}
	if notice := DescribeFileResults([]FileResult{result}); !strings.Contains(notice, result.Backup) {
		t.Errorf("the notice should name the backup, got %q", notice)
	}
}

// La configuracion se escribe al arrancar el servicio, no en un paso aparte.
func TestStartWritesTheServiceConfiguration(t *testing.T) {
	withServiceFiles(t, "redis", ManagedFile{Path: "redis/redis.conf", Content: "maxmemory 64mb\n"})

	m := newTestManager(t, &fakeRunner{})
	_, files, err := m.Start(context.Background(), "redis", false)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if len(files) != 1 || files[0].State != FileWritten {
		t.Fatalf("expected the config file to be written, got %#v", files)
	}
	if got := readFile(t, filepath.Join(m.dir, "redis", "redis.conf")); got != "maxmemory 64mb\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

// Un servicio sin configuracion se comporta exactamente igual que antes.
func TestStartOnAServiceWithoutConfigurationReportsNothing(t *testing.T) {
	m := newTestManager(t, &fakeRunner{})

	_, files, err := m.Start(context.Background(), "mailpit", false)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("mailpit declares no configuration, got %#v", files)
	}
	if notice := DescribeFileResults(files); notice != "" {
		t.Errorf("expected no notice, got %q", notice)
	}
}

// **El bug que esta historia arregla de paso.** Consultar el estado no puede tener
// efectos, y hasta ahora reescribia el compose en cada llamada.
func TestStatusDoesNotWriteAnything(t *testing.T) {
	withServiceFiles(t, "redis", ManagedFile{Path: "redis/redis.conf", Content: "maxmemory 64mb\n"})

	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", "ps output"}})
	ctx := context.Background()

	if _, _, err := m.Start(ctx, "redis", false); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Las dos ediciones que Status no debe tocar.
	compose := m.composeFile
	conf := filepath.Join(m.dir, "redis", "redis.conf")
	if err := os.WriteFile(compose, []byte("# editado a mano\n"), 0o644); err != nil {
		t.Fatalf("edit compose: %v", err)
	}
	if err := os.WriteFile(conf, []byte("maxmemory 1gb\n"), 0o644); err != nil {
		t.Fatalf("edit conf: %v", err)
	}

	if _, err := m.Status(ctx, ""); err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if got := readFile(t, compose); got != "# editado a mano\n" {
		t.Errorf("Status rewrote the compose: %q", got)
	}
	if got := readFile(t, conf); got != "maxmemory 1gb\n" {
		t.Errorf("Status rewrote the configuration: %q", got)
	}
}

// Sin nada arrancado, Status responde en vez de fallar o de crear el stack para
// que `docker compose ps` devuelva una tabla vacia.
func TestStatusOnAnUntouchedInstallAnswersInsteadOfFailing(t *testing.T) {
	r := &fakeRunner{}
	m := newTestManager(t, r)

	out, err := m.Status(context.Background(), "")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !strings.Contains(out, "no shared services have been started yet") {
		t.Errorf("expected an explanation, got %q", out)
	}
	if len(r.calls) != 0 {
		t.Errorf("nothing should have been run, got %v", r.calls)
	}
	if _, err := os.Stat(m.composeFile); !os.IsNotExist(err) {
		t.Error("Status created the compose file")
	}
}

// El compose es de DevHerd, no del usuario: es el catalogo de lo que se ofrece.
// Congelarlo con la edicion de alguien haria que una version nueva del binario no
// pudiera ofrecer un servicio nuevo.
func TestStartAlwaysRegeneratesTheCompose(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", ""}})
	ctx := context.Background()

	if _, _, err := m.Start(ctx, "redis", false); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := os.WriteFile(m.composeFile, []byte("# borrado el catalogo\n"), 0o644); err != nil {
		t.Fatalf("edit compose: %v", err)
	}

	if _, _, err := m.Start(ctx, "redis", false); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if got := readFile(t, m.composeFile); !strings.Contains(got, "mailpit") {
		t.Errorf("the compose should be regenerated from the catalog, got:\n%s", got)
	}
}
