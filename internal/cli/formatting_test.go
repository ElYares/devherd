package cli

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/doctor"
	"github.com/devherd/devherd/internal/observe"
	"github.com/devherd/devherd/internal/preflight"
)

func TestTruncateObserveText(t *testing.T) {
	cases := []struct {
		name  string
		value string
		limit int
		want  string
	}{
		{"sin limite devuelve todo", "mensaje largo de verdad", 0, "mensaje largo de verdad"},
		{"limite negativo devuelve todo", "mensaje", -5, "mensaje"},
		{"cabe justo", "12345", 5, "12345"},
		{"recorta con puntos", "1234567890", 8, "12345..."},
		{"limite menor que los puntos no los pone", "1234567890", 3, "123"},
		{"recorta espacios de los extremos", "   con espacios   ", 0, "con espacios"},
		{"no deja espacio antes de los puntos", "hola mundo cruel", 8, "hola..."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateObserveText(tc.value, tc.limit); got != tc.want {
				t.Fatalf("truncateObserveText(%q, %d) = %q, want %q", tc.value, tc.limit, got, tc.want)
			}
		})
	}
}

func TestSupportedAlertKind(t *testing.T) {
	for _, kind := range []string{"new-issue", "error-rate", "container-exit", "container-restart"} {
		if !supportedAlertKind(kind) {
			t.Fatalf("expected %q to be supported", kind)
		}
	}
	for _, kind := range []string{"", "New-Issue", "issue", "error_rate", "cualquier-cosa"} {
		if supportedAlertKind(kind) {
			t.Fatalf("expected %q to be rejected", kind)
		}
	}
}

func TestParseObserveDurationSeconds(t *testing.T) {
	if got, err := parseObserveDurationSeconds(""); err != nil || got != 300 {
		t.Fatalf("empty window should default to 5m, got %d (err %v)", got, err)
	}
	if got, err := parseObserveDurationSeconds("  90s  "); err != nil || got != 90 {
		t.Fatalf("expected 90 seconds, got %d (err %v)", got, err)
	}
	for _, value := range []string{"0s", "-1m", "cinco minutos", "5"} {
		if _, err := parseObserveDurationSeconds(value); err == nil {
			t.Fatalf("expected %q to be rejected as a window", value)
		}
	}
}

// El cero es la diferencia deliberada entre los dos parsers: una ventana de cero
// no significa nada, pero un cooldown de cero significa "entregar siempre".
func TestObserveCooldownAcceptsZeroButWindowDoesNot(t *testing.T) {
	got, err := parseObserveCooldownSeconds("0s")
	if err != nil {
		t.Fatalf("cooldown 0s should be valid: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	if _, err := parseObserveDurationSeconds("0s"); err == nil {
		t.Fatal("a window of 0s must be rejected")
	}
	if _, err := parseObserveCooldownSeconds(""); err == nil {
		t.Fatal("an empty cooldown must be rejected, unlike an empty window")
	}
}

func TestEmptyAsAll(t *testing.T) {
	if got := emptyAsAll("   "); got != "(all)" {
		t.Fatalf("expected (all) for blank input, got %q", got)
	}
	if got := emptyAsAll("  demo "); got != "demo" {
		t.Fatalf("expected the trimmed value, got %q", got)
	}
}

func TestFormatObservePayloadValue(t *testing.T) {
	if got := formatObservePayloadValue("texto plano"); got != "texto plano" {
		t.Fatalf("strings should pass through untouched, got %q", got)
	}
	if got := formatObservePayloadValue(map[string]any{"user": 42}); got != `{"user":42}` {
		t.Fatalf("expected compact JSON, got %q", got)
	}
	if got := formatObservePayloadValue([]any{"a", 1}); got != `["a",1]` {
		t.Fatalf("expected compact JSON for slices, got %q", got)
	}
	// json.Marshal no sabe serializar NaN, que es justo lo que llega cuando un SDK
	// mete una metrica rota en el payload. Cae al formato de Go y no pierde el dato.
	if got := formatObservePayloadValue(math.NaN()); got != "NaN" {
		t.Fatalf("expected the Go fallback for unmarshalable values, got %q", got)
	}
}

func TestStatusLabel(t *testing.T) {
	cases := map[doctor.Status]string{
		doctor.StatusOK:              "OK",
		doctor.StatusWarn:            "WARN",
		doctor.StatusFail:            "FAIL",
		doctor.Status("desconocido"): "INFO",
	}
	for status, want := range cases {
		if got := statusLabel(status); got != want {
			t.Fatalf("statusLabel(%v) = %q, want %q", status, got, want)
		}
	}
}

func TestServiceActionShort(t *testing.T) {
	cases := map[string]string{
		"start":       "Start a shared service",
		"stop":        "Stop a shared service",
		"status":      "Show status for a shared service",
		"desconocido": "Manage a shared service",
		"":            "Manage a shared service",
	}
	for action, want := range cases {
		if got := serviceActionShort(action); got != want {
			t.Fatalf("serviceActionShort(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestDescribeProjectSource(t *testing.T) {
	manifest := compose.Project{Root: "/proyectos/demo", Source: compose.ProjectSourceManifest}
	if got := describeProjectSource(manifest); got != filepath.Join("/proyectos/demo", ".devherd.yml") {
		t.Fatalf("expected the manifest path, got %q", got)
	}

	if got := describeProjectSource(compose.Project{Root: "/proyectos/demo"}); got != "compose autodetect" {
		t.Fatalf("expected the autodetect label, got %q", got)
	}
}

func TestDescribeEnvFile(t *testing.T) {
	if got := describeEnvFile(compose.Project{EnvFile: "/proyectos/demo/.env.local"}); got != "/proyectos/demo/.env.local" {
		t.Fatalf("an explicit env file wins, got %q", got)
	}

	root := t.TempDir()
	if got := describeEnvFile(compose.Project{Root: root}); got != "compose default (none detected)" {
		t.Fatalf("expected the none-detected label, got %q", got)
	}

	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("APP_ENV=local\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if got := describeEnvFile(compose.Project{Root: root}); !strings.Contains(got, envPath) {
		t.Fatalf("expected the detected .env path, got %q", got)
	}
}

func TestWriteObserveTimelineSaysWhatIsMissing(t *testing.T) {
	var out bytes.Buffer
	writeObserveTimeline(&out, observe.Timeline{
		Event: observe.EventRecord{
			EventID: "abc123",
			Project: "demo",
			IssueID: 7,
			Message: "boom",
		},
	})

	text := out.String()
	for _, want := range []string{"Event: abc123", "Project: demo", "Issue: 7", "Message: boom", "- none", "- none captured"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in the timeline output:\n%s", want, text)
		}
	}
	// Sin excepcion ni culprit no se imprimen etiquetas vacias.
	if strings.Contains(text, "Exception:") || strings.Contains(text, "Culprit:") {
		t.Fatalf("empty fields should be omitted:\n%s", text)
	}
}

func TestWriteObservePayloadShowsOnlyExtraKeys(t *testing.T) {
	var out bytes.Buffer
	writeObservePayload(&out, `{"message":"boom","tags":{"env":"local"},"context":"extra"}`)

	text := out.String()
	if !strings.Contains(text, "Payload:") {
		t.Fatalf("expected a payload block:\n%s", text)
	}
	if !strings.Contains(text, "context: extra") {
		t.Fatalf("expected the context key:\n%s", text)
	}
	// `message` ya tiene columna propia: mostrarlo aqui seria duplicar.
	if strings.Contains(text, "- message:") {
		t.Fatalf("keys with their own column must not repeat here:\n%s", text)
	}
	// Claves ordenadas, para que la salida no dependa del recorrido del mapa.
	if strings.Index(text, "context:") > strings.Index(text, "tags:") {
		t.Fatalf("payload keys should be sorted:\n%s", text)
	}
}

func TestWriteObservePayloadStaysQuietWhenThereIsNothingExtra(t *testing.T) {
	var out bytes.Buffer
	writeObservePayload(&out, `{"message":"boom"}`)
	if out.Len() != 0 {
		t.Fatalf("expected no output without extra keys, got %q", out.String())
	}

	out.Reset()
	writeObservePayload(&out, "no es json")
	if out.Len() != 0 {
		t.Fatalf("expected no output for an invalid payload, got %q", out.String())
	}
}

func TestWritePreflightReportHidesOKUnlessAsked(t *testing.T) {
	report := preflight.Report{
		Project: compose.Project{Root: "/proyectos/demo"},
		Findings: []preflight.Finding{
			{Severity: preflight.SeverityOK, Name: "docker", Message: "docker esta arriba"},
			{Severity: preflight.SeverityFail, Name: "puerto", Message: "9777 ocupado"},
		},
	}

	var quiet bytes.Buffer
	writePreflightReport(&quiet, report, false)
	if strings.Contains(quiet.String(), "docker esta arriba") {
		t.Fatalf("OK checks should be hidden by default:\n%s", quiet.String())
	}
	if !strings.Contains(quiet.String(), "9777 ocupado") {
		t.Fatalf("failures must always show:\n%s", quiet.String())
	}

	var verbose bytes.Buffer
	writePreflightReport(&verbose, report, true)
	if !strings.Contains(verbose.String(), "docker esta arriba") {
		t.Fatalf("OK checks should show when asked:\n%s", verbose.String())
	}
}
