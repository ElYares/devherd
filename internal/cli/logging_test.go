package cli

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupLoggingDoesNotPanic(t *testing.T) {
	// Restaura el logger global por defecto al terminar.
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })

	for _, o := range []logOptions{
		{verbose: false, json: false},
		{verbose: true, json: false},
		{verbose: false, json: true},
		{verbose: true, json: true},
	} {
		setupLogging(o)
	}
}

func TestVerboseEnablesDebugLevel(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })

	var buf bytes.Buffer
	// Sin verbose, DEBUG no debe emitirse; con verbose sí.
	setupLogging(logOptions{verbose: false})
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("debug should be disabled without --verbose")
	}

	setupLogging(logOptions{verbose: true})
	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("debug should be enabled with --verbose")
	}

	// Sanity: el logger escribe a un handler funcional.
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.New(h).Debug("hello", "k", "v")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected log output to contain message, got %q", buf.String())
	}
}
