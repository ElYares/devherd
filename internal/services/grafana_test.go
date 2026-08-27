package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Los tres archivos de provisioning van en las rutas que el compose monta. Una
// ruta mal puesta no falla al arrancar: Grafana levanta igual y muestra un tablero
// vacio, que es la peor forma de fallar porque parece que funciona.
func TestGrafanaProvisionsDatasourceAndDashboard(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	expected := []string{
		"grafana/dashboards/devherd.yml",
		"grafana/dashboards/devherd/devherd-observe.json",
		"grafana/datasources/prometheus.yml",
	}
	if len(paths) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, paths)
	}
	for i, want := range expected {
		if paths[i] != want {
			t.Errorf("file %d: expected %q, got %q", i, want, paths[i])
		}
	}
}

// Se apunta a Prometheus por su alias de red, no por IP: las dos comparten
// infra_net y Docker resuelve el nombre. Una IP cambiaria al recrear la red, que
// es la misma trampa que ya costo un falso positivo en Observe.
func TestGrafanaDatasourceUsesTheNetworkAlias(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var datasource string
	for _, file := range files {
		if strings.HasSuffix(file.Path, "datasources/prometheus.yml") {
			datasource = file.Content
		}
	}
	if datasource == "" {
		t.Fatal("the datasource file is missing")
	}

	if !strings.Contains(datasource, "url: http://prometheus:9090") {
		t.Errorf("the datasource should target the network alias, got:\n%s", datasource)
	}
	// Una IP en el datasource es la forma de que deje de funcionar sola.
	for _, line := range strings.Split(datasource, "\n") {
		if strings.Contains(line, "url:") && strings.Contains(line, "172.") {
			t.Errorf("the datasource must not hardcode an IP: %q", line)
		}
	}
}

// **El criterio que decide si empaquetar Grafana valio la pena.** Con datasource y
// sin tableros, el usuario se queda exactamente donde estaba.
func TestGrafanaDashboardQueriesTheObserveMetrics(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var raw string
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".json") {
			raw = file.Content
		}
	}
	if raw == "" {
		t.Fatal("the dashboard is missing")
	}

	var dashboard struct {
		UID    string `json:"uid"`
		Title  string `json:"title"`
		Panels []struct {
			Title   string `json:"title"`
			Targets []struct {
				Expr       string `json:"expr"`
				Datasource struct {
					UID string `json:"uid"`
				} `json:"datasource"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(raw), &dashboard); err != nil {
		t.Fatalf("the dashboard is not valid JSON: %v", err)
	}

	if len(dashboard.Panels) == 0 {
		t.Fatal("a dashboard with no panels leaves the user exactly where they were")
	}

	// Cada panel consulta algo, y contra el datasource que se provisiona.
	queries := make([]string, 0, len(dashboard.Panels))
	for _, panel := range dashboard.Panels {
		if len(panel.Targets) == 0 {
			t.Errorf("panel %q has no query", panel.Title)

			continue
		}
		for _, target := range panel.Targets {
			if target.Expr == "" {
				t.Errorf("panel %q has an empty query", panel.Title)
			}
			if target.Datasource.UID != "devherd-prometheus" {
				t.Errorf("panel %q points at %q instead of the provisioned datasource",
					panel.Title, target.Datasource.UID)
			}
			queries = append(queries, target.Expr)
		}
	}

	// Las tres familias que cuentan la historia: donde duele, que se reinicia, y si
	// el collector estuvo escuchando.
	joined := strings.Join(queries, " ")
	for _, metric := range []string{
		"devherd_observe_issues",
		"devherd_observe_container_restarts_total",
		"devherd_observe_collector_gap_seconds",
	} {
		if !strings.Contains(joined, metric) {
			t.Errorf("no panel queries %s", metric)
		}
	}
}

// Grafana sin Prometheus arranca y muestra paneles vacios. Declararlo permite
// avisar antes, en vez de que se descubra al abrir el tablero.
func TestGrafanaDependsOnPrometheus(t *testing.T) {
	if got := DependsOn("grafana"); got != "prometheus" {
		t.Errorf("expected grafana to depend on prometheus, got %q", got)
	}
	for _, service := range []string{"redis", "mailpit", "prometheus"} {
		if got := DependsOn(service); got != "" {
			t.Errorf("%s should not declare a dependency, got %q", service, got)
		}
	}
}

// Arrancarlo escribe los tres archivos donde el compose los monta.
func TestStartGrafanaWritesItsProvisioning(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", ""}})

	_, files, err := m.Start(context.Background(), "grafana", StartOptions{})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected three provisioning files, got %#v", files)
	}

	for _, relative := range []string{
		"grafana/datasources/prometheus.yml",
		"grafana/dashboards/devherd.yml",
		"grafana/dashboards/devherd/devherd-observe.json",
	} {
		if _, err := os.Stat(filepath.Join(m.dir, filepath.FromSlash(relative))); err != nil {
			t.Errorf("%s was not written: %v", relative, err)
		}
	}
}

// Sin login: es un entorno local y un login que nadie recuerda es friccion sin
// seguridad. El puerto solo escucha en loopback, asi que no hay superficie que
// proteger.
func TestGrafanaDoesNotAskForCredentials(t *testing.T) {
	for _, want := range []string{
		`GF_AUTH_ANONYMOUS_ENABLED: "true"`,
		`GF_AUTH_DISABLE_LOGIN_FORM: "true"`,
	} {
		if !strings.Contains(composeContent, want) {
			t.Errorf("the compose should set %s", want)
		}
	}
	if !strings.Contains(composeContent, `"127.0.0.1:3000:3000"`) {
		t.Error("grafana must publish its port on loopback only; anonymous access depends on it")
	}
}

// Grafana entra al catalogo sin desplazar a nadie.
func TestGrafanaIsPartOfTheCatalog(t *testing.T) {
	supported := SupportedServices()
	for _, want := range []string{"redis", "mailpit", "prometheus", "grafana"} {
		if !contains(supported, want) {
			t.Errorf("%s should be a supported service, got %v", want, supported)
		}
	}
}
