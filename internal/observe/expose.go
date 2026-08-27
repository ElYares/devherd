package observe

import (
	"fmt"
	"sort"
	"strings"
)

// MetricsContentType es lo que espera Prometheus. La version del formato va en el
// header, no es adorno: un scraper que no la reconoce cae a texto plano y deja de
// interpretar los tipos.
const MetricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// metricsPrefix agrupa todas las series bajo un nombre. Prometheus no tiene
// espacios de nombres: el prefijo del nombre es todo lo que hay.
const metricsPrefix = "devherd_observe_"

// FormatMetrics escribe el snapshot en el formato de exposicion de Prometheus.
//
// Se escribe a mano y no con `prometheus/client_golang` a proposito. El formato
// es una linea por muestra —nombre, etiquetas, valor— con dos lineas de metadatos
// por familia. Son unas decenas de lineas de codigo, contra una dependencia que
// arrastra un registro global, un recolector de metricas del runtime y su propia
// superficie de API. Seria la cuarta dependencia directa de un proyecto que tiene
// tres y trata eso como decision de diseño.
func FormatMetrics(snapshot MetricsSnapshot) string {
	var b strings.Builder

	// Las familias van completas y en orden: todas las muestras de una metrica
	// juntas, detras de su HELP y su TYPE. Intercalarlas es un error de formato,
	// no un detalle de presentacion.
	writeFamily(&b, "issues", "gauge",
		"Issues currently grouped by DevHerd Observe.",
		issueSamples(snapshot, func(c IssueCount) int { return c.Issues }))

	writeFamily(&b, "events_total", "counter",
		"Events received by DevHerd Observe since the database was created.",
		issueSamples(snapshot, func(c IssueCount) int { return c.Events }))

	writeFamily(&b, "container_restarts_total", "counter",
		"Restarts observed per container.",
		containerSamples(snapshot))

	// Sin corridas registradas no se publica la muestra en vez de publicar un 0.
	// Un 0 significa "el collector esta caido"; la ausencia significa "no se sabe",
	// y confundirlas dispara alertas falsas el primer dia de uso.
	uptime := []sample{}
	if snapshot.CollectorRunning {
		uptime = append(uptime, sample{value: snapshot.CollectorUptime.Seconds()})
	}
	writeFamily(&b, "collector_uptime_seconds", "gauge",
		"Seconds the current collector run has been listening.", uptime)

	writeFamily(&b, "collector_gap_seconds", "gauge",
		"Seconds in the last 24h with no collector listening; issues are absent from those windows because nobody received them.",
		[]sample{{value: snapshot.GapSeconds24h}})

	return b.String()
}

// sample es una muestra: sus etiquetas y su valor.
type sample struct {
	labels map[string]string
	value  float64
}

// writeFamily escribe una familia completa. Una familia sin muestras conserva su
// HELP y su TYPE: le dice al scraper que la metrica existe y ahora no tiene datos,
// que es distinto de que la metrica no exista.
func writeFamily(b *strings.Builder, name, kind, help string, samples []sample) {
	full := metricsPrefix + name

	fmt.Fprintf(b, "# HELP %s %s\n", full, escapeHelp(help))
	fmt.Fprintf(b, "# TYPE %s %s\n", full, kind)
	for _, s := range samples {
		fmt.Fprintf(b, "%s%s %s\n", full, formatLabels(s.labels), formatValue(s.value))
	}
}

func issueSamples(snapshot MetricsSnapshot, pick func(IssueCount) int) []sample {
	samples := make([]sample, 0, len(snapshot.Issues))
	for _, count := range snapshot.Issues {
		samples = append(samples, sample{
			labels: map[string]string{"project": count.Project, "level": count.Level},
			value:  float64(pick(count)),
		})
	}

	return samples
}

func containerSamples(snapshot MetricsSnapshot) []sample {
	samples := make([]sample, 0, len(snapshot.Containers))
	for _, count := range snapshot.Containers {
		samples = append(samples, sample{
			labels: map[string]string{
				"project":   count.Project,
				"service":   count.Service,
				"container": count.Name,
			},
			value: float64(count.Restarts),
		})
	}

	return samples
}

// formatLabels escribe el bloque de etiquetas, ordenado por nombre para que dos
// scrapes del mismo estado den el mismo texto.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+`="`+escapeLabelValue(labels[name])+`"`)
	}

	return "{" + strings.Join(parts, ",") + "}"
}

// escapeLabelValue protege el valor de una etiqueta. Un nombre de proyecto con una
// comilla o una barra invertida romperia el documento entero, y los nombres de
// proyecto salen de rutas del disco, que es donde vive cualquier caracter.
func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

	return replacer.Replace(value)
}

// escapeHelp protege el texto de ayuda, que admite menos escapes que una etiqueta:
// solo la barra invertida y el salto de linea.
func escapeHelp(help string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`)

	return replacer.Replace(help)
}

// formatValue escribe el numero. Los enteros salen sin decimales para que el texto
// se lea, y los demas con la precision justa para no perder informacion.
func formatValue(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}

	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}
