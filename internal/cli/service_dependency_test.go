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
