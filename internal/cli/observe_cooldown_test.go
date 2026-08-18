package cli

import (
	"testing"

	"github.com/devherd/devherd/internal/observe"
)

func TestParseObserveCooldownSecondsAcceptsZero(t *testing.T) {
	got, err := parseObserveCooldownSeconds("0")
	if err != nil {
		t.Fatalf("parseObserveCooldownSeconds(0) returned error: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 seconds, got %d", got)
	}
}

func TestParseObserveCooldownSecondsRejectsNegative(t *testing.T) {
	if _, err := parseObserveCooldownSeconds("-1m"); err == nil {
		t.Fatal("expected an error for a negative cooldown")
	}
}

func TestParseObserveCooldownSecondsParsesDurations(t *testing.T) {
	got, err := parseObserveCooldownSeconds("15m")
	if err != nil {
		t.Fatalf("parseObserveCooldownSeconds(15m) returned error: %v", err)
	}
	if got != 900 {
		t.Fatalf("expected 900 seconds, got %d", got)
	}
}

// El default de error-rate es la ventana, no un numero fijo: la ventana ya expresa
// el ritmo que le interesa al usuario.
func TestDefaultCooldownFollowsTheWindowOnlyForErrorRate(t *testing.T) {
	if got := observe.DefaultCooldownSeconds("error-rate", 600); got != 600 {
		t.Fatalf("expected error-rate to default to its window, got %d", got)
	}
	for _, kind := range []string{"new-issue", "container-exit", "container-restart"} {
		if got := observe.DefaultCooldownSeconds(kind, 600); got != 900 {
			t.Fatalf("expected %s to default to 900, got %d", kind, got)
		}
	}
}
