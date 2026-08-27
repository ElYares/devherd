package cli

import (
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/services"
)

// Grafana sin Prometheus arranca y muestra paneles vacios, que es la peor forma de
// fallar: parece que funciona. El aviso sale **antes** de arrancar, no despues de
// que el usuario abra el tablero y no entienda nada.
func TestMissingDependencyWarningNamesTheFixes(t *testing.T) {
	warning := dependencyWarning("grafana", "prometheus")

	for _, want := range []string{
		"prometheus is not running",
		"empty panels",
		"devherd service start prometheus",
		"your own prometheus",
	} {
		if !strings.Contains(warning, want) {
			t.Errorf("the warning should mention %q, got:\n%s", want, warning)
		}
	}
}

// Solo Grafana declara dependencia; el resto no genera aviso.
func TestOnlyGrafanaDeclaresADependency(t *testing.T) {
	for _, service := range []string{"redis", "mailpit", "prometheus"} {
		if services.DependsOn(service) != "" {
			t.Errorf("%s should not declare a dependency", service)
		}
	}
	if services.DependsOn("grafana") != "prometheus" {
		t.Error("grafana depends on prometheus")
	}
}

// El dominio se ofrece como alternativa y tiene que llevar la ruta y el token: sin
// eso, "tambien en jupyter.localhost" manda a una pagina que pide un token que el
// usuario acaba de tener delante.
func TestSharedServiceURLCarriesPathAndToken(t *testing.T) {
	got := sharedServiceURL("jupyter.localhost", "http://127.0.0.1:8888/lab?token=abc123")
	if got != "http://jupyter.localhost/lab?token=abc123" {
		t.Errorf("expected the path and query to travel, got %q", got)
	}
}

// Un servicio sin ruta ni token queda en la raiz del dominio.
func TestSharedServiceURLWithoutPath(t *testing.T) {
	got := sharedServiceURL("grafana.localhost", "http://127.0.0.1:3000")
	if got != "http://grafana.localhost" {
		t.Errorf("expected the bare domain, got %q", got)
	}
}

// Una URL rota no puede tumbar el mensaje: el servicio ya esta arriba.
func TestSharedServiceURLSurvivesAMalformedInput(t *testing.T) {
	if got := sharedServiceURL("jupyter.localhost", "://roto"); got != "http://jupyter.localhost" {
		t.Errorf("expected a usable fallback, got %q", got)
	}
}
