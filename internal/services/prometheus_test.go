package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// **El criterio que decide si empaquetar Prometheus vale la pena**: el collector
// queda apuntado sin que el usuario escriba una linea de YAML.
func TestPrometheusConfigTargetsTheCollector(t *testing.T) {
	files, err := ServiceFiles("prometheus", ServiceOptions{CollectorAddr: "172.20.0.1:9777"})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one configuration file, got %#v", files)
	}
	if files[0].Path != "prometheus/prometheus.yml" {
		t.Errorf("unexpected path %q", files[0].Path)
	}

	content := files[0].Content
	if !strings.Contains(content, `targets: ["172.20.0.1:9777"]`) {
		t.Errorf("the collector is not targeted, got:\n%s", content)
	}
	if !strings.Contains(content, "metrics_path: /metrics") {
		t.Errorf("the scrape path should be /metrics, got:\n%s", content)
	}
	// La plantilla no puede quedar sin renderizar.
	if strings.Contains(content, "{{") {
		t.Errorf("the template was not rendered:\n%s", content)
	}
}

// Sin direccion no se escribe una configuracion rota: un target vacio deja un
// Prometheus que arranca y no sirve, que es peor que uno que no arranca.
func TestPrometheusConfigRequiresTheCollectorAddress(t *testing.T) {
	for _, addr := range []string{"", "   "} {
		if _, err := ServiceFiles("prometheus", ServiceOptions{CollectorAddr: addr}); err == nil {
			t.Errorf("expected an error for address %q", addr)
		}
	}
}

// Prometheus es el unico servicio que necesita saber donde escucha el collector.
func TestNeedsCollectorOnlyForPrometheus(t *testing.T) {
	if !NeedsCollector("prometheus") {
		t.Error("prometheus needs the collector address")
	}
	for _, service := range []string{"redis", "mailpit"} {
		if NeedsCollector(service) {
			t.Errorf("%s does not need the collector address", service)
		}
	}
}

// Arrancarlo escribe la configuracion junto al compose, en la ruta que el compose
// monta como volumen.
func TestStartPrometheusWritesItsConfiguration(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", ""}})

	_, files, err := m.Start(context.Background(), "prometheus", StartOptions{
		CollectorAddr: "172.20.0.1:9777",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(files) != 1 || files[0].State != FileWritten {
		t.Fatalf("expected the configuration to be written, got %#v", files)
	}

	path := filepath.Join(m.dir, "prometheus", "prometheus.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written configuration: %v", err)
	}
	if !strings.Contains(string(data), "172.20.0.1:9777") {
		t.Errorf("the written configuration does not target the collector:\n%s", data)
	}
}

// La edicion del usuario sobrevive, igual que para cualquier otro servicio: el
// criterio se hereda de HU-011 y hay que comprobar que Prometheus no lo rompe.
func TestStartPrometheusKeepsTheUserConfiguration(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", "", "", "", ""}})
	ctx := context.Background()
	opts := StartOptions{CollectorAddr: "172.20.0.1:9777"}

	if _, _, err := m.Start(ctx, "prometheus", opts); err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}

	path := filepath.Join(m.dir, "prometheus", "prometheus.yml")
	if err := os.WriteFile(path, []byte("global:\n  scrape_interval: 1s\n"), 0o644); err != nil {
		t.Fatalf("simulate a user edit: %v", err)
	}

	_, files, err := m.Start(ctx, "prometheus", opts)
	if err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if len(files) != 1 || files[0].State != FileKept {
		t.Fatalf("expected the user configuration to be kept, got %#v", files)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read configuration: %v", err)
	}
	if string(data) != "global:\n  scrape_interval: 1s\n" {
		t.Errorf("the user configuration was overwritten:\n%s", data)
	}
}

// El compose publica el puerto solo en loopback, como redis y mailpit: un
// Prometheus escuchando en todas las interfaces expone la telemetria local a la
// red donde este la maquina.
func TestPrometheusIsPublishedOnLoopbackOnly(t *testing.T) {
	if !strings.Contains(composeContent, `"127.0.0.1:9090:9090"`) {
		t.Error("prometheus should publish its port on 127.0.0.1 only")
	}
	for _, line := range strings.Split(composeContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `- "`) && !strings.Contains(trimmed, "127.0.0.1:") {
			t.Errorf("every published port must bind loopback, found: %q", trimmed)
		}
	}
}

// La imagen va fijada. La leccion de golangci-lint aplica igual: el sintoma de una
// version que cambio sola no se parece a lo que es.
func TestPrometheusImageIsPinned(t *testing.T) {
	for _, line := range strings.Split(composeContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "image:") {
			continue
		}
		if strings.HasSuffix(trimmed, ":latest") || !strings.Contains(trimmed, ":") {
			t.Errorf("image is not pinned to a version: %q", trimmed)
		}
	}
}

// Prometheus entra al catalogo sin desplazar a nadie, y sigue siendo opcional.
func TestPrometheusIsPartOfTheCatalog(t *testing.T) {
	supported := SupportedServices()
	for _, want := range []string{"redis", "mailpit", "prometheus"} {
		if !contains(supported, want) {
			t.Errorf("%s should be a supported service, got %v", want, supported)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}

	return false
}
