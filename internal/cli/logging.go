package cli

import (
	"log/slog"
	"os"
)

// logOptions controla la configuración del logger global de diagnóstico.
type logOptions struct {
	verbose bool
	json    bool
}

// setupLogging configura el logger global de slog usado para diagnósticos.
// Los diagnósticos van a stderr; la salida "de producto" sigue en stdout vía
// cmd.OutOrStdout(). Con --verbose el nivel baja a DEBUG; con --log-json se
// emite en formato JSON (útil para scripting o agregadores).
func setupLogging(o logOptions) {
	level := slog.LevelInfo
	if o.verbose {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if o.json {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}
