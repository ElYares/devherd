package observe

import (
	"strings"
	"testing"
	"time"
)

func demoSnapshot() MetricsSnapshot {
	return MetricsSnapshot{
		TakenAt: time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC),
		Issues: []IssueCount{
			{Project: "aang-server", Level: "error", Issues: 3, Events: 41},
			{Project: "aang-server", Level: "warning", Issues: 1, Events: 5},
			{Project: "tl-mas-server", Level: "error", Issues: 2, Events: 9},
		},
		Containers: []ContainerCount{
			{Project: "aang-server", Service: "web", Name: "aang_web", Restarts: 3},
			{Project: "aang-server", Service: "queue", Name: "aang_queue", Restarts: 0},
		},
		CollectorRunning: true,
		CollectorUptime:  90 * time.Minute,
		GapSeconds24h:    14400,
	}
}

// Las cinco familias salen con su HELP, su TYPE y el prefijo comun. Prometheus no
// tiene espacios de nombres: el prefijo del nombre es todo lo que hay.
func TestFormatMetricsWritesEveryFamilyWithMetadata(t *testing.T) {
	out := FormatMetrics(demoSnapshot())

	families := []string{
		"devherd_observe_issues",
		"devherd_observe_events_total",
		"devherd_observe_container_restarts_total",
		"devherd_observe_collector_uptime_seconds",
		"devherd_observe_collector_gap_seconds",
	}
	for _, family := range families {
		if !strings.Contains(out, "# HELP "+family+" ") {
			t.Errorf("missing HELP for %s", family)
		}
		if !strings.Contains(out, "# TYPE "+family+" ") {
			t.Errorf("missing TYPE for %s", family)
		}
	}
}

// Las muestras llevan las etiquetas ordenadas y los valores que trae el snapshot.
func TestFormatMetricsWritesLabelledSamples(t *testing.T) {
	out := FormatMetrics(demoSnapshot())

	expected := []string{
		`devherd_observe_issues{level="error",project="aang-server"} 3`,
		`devherd_observe_events_total{level="error",project="aang-server"} 41`,
		`devherd_observe_container_restarts_total{container="aang_web",project="aang-server",service="web"} 3`,
		`devherd_observe_collector_uptime_seconds 5400`,
		`devherd_observe_collector_gap_seconds 14400`,
	}
	for _, line := range expected {
		if !strings.Contains(out, line) {
			t.Errorf("expected the output to contain:\n  %s\ngot:\n%s", line, out)
		}
	}
}

// Todas las muestras de una familia van juntas detras de su HELP y su TYPE.
// Intercalarlas es un error de formato, no un detalle de presentacion.
func TestFormatMetricsKeepsEachFamilyTogether(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(FormatMetrics(demoSnapshot())), "\n")

	seen := map[string]bool{}
	current := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP ") {
			current = strings.Fields(line)[2]
			if seen[current] {
				t.Fatalf("family %s is declared twice; its samples are interleaved", current)
			}
			seen[current] = true

			continue
		}
		if strings.HasPrefix(line, "# TYPE ") {
			continue
		}
		name := strings.SplitN(strings.SplitN(line, "{", 2)[0], " ", 2)[0]
		if name != current {
			t.Errorf("sample %q appears under family %q", name, current)
		}
	}
}

// Sin corridas registradas, la muestra de uptime **no se publica**. Un 0 significa
// "el collector esta caido"; la ausencia significa "no se sabe", y confundirlas
// dispara alertas falsas el primer dia de uso.
func TestFormatMetricsOmitsUptimeWhenNothingIsKnown(t *testing.T) {
	out := FormatMetrics(MetricsSnapshot{})

	if !strings.Contains(out, "# TYPE devherd_observe_collector_uptime_seconds gauge") {
		t.Error("the family should still be declared: the metric exists, it just has no data")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "devherd_observe_collector_uptime_seconds ") {
			t.Errorf("uptime should not be sampled when no session is running, got %q", line)
		}
	}
}

// Los nombres de proyecto salen de rutas del disco, que es donde vive cualquier
// caracter. Una comilla sin escapar rompe el documento entero.
func TestFormatMetricsEscapesLabelValues(t *testing.T) {
	out := FormatMetrics(MetricsSnapshot{
		Issues: []IssueCount{{Project: `we"ird\path`, Level: "error", Issues: 1}},
	})

	if !strings.Contains(out, `project="we\"ird\\path"`) {
		t.Errorf("label value is not escaped, got:\n%s", out)
	}
}

// Dos formateos del mismo estado tienen que dar el mismo texto. El recorrido de un
// mapa en Go no tiene orden, y sin fijarlo las etiquetas saldrian barajadas.
func TestFormatMetricsIsDeterministic(t *testing.T) {
	first := FormatMetrics(demoSnapshot())
	for i := 0; i < 20; i++ {
		if again := FormatMetrics(demoSnapshot()); again != first {
			t.Fatal("the same snapshot produced two different documents")
		}
	}
}

// Los enteros salen sin decimales, para que el texto se pueda leer a ojo.
func TestFormatValueKeepsIntegersReadable(t *testing.T) {
	cases := map[float64]string{
		0:    "0",
		5400: "5400",
		1.5:  "1.5",
		0.25: "0.25",
		-3:   "-3",
	}
	for value, want := range cases {
		if got := formatValue(value); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", value, got, want)
		}
	}
}
