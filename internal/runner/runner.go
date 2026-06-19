// Package runner centraliza la ejecución de comandos externos (docker, etc.)
// detrás de una interfaz, para poder inyectar un doble en tests y unificar la
// semántica de captura de salida que antes estaba triplicada en compose,
// services, proxy y doctor.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Runner ejecuta un comando externo y devuelve su salida combinada (stdout+stderr)
// ya recortada. En caso de error, si hubo salida la usa como mensaje; si no,
// devuelve el error original del proceso.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
}

// Cmd es la implementación real basada en os/exec.
type Cmd struct {
	// Timeout opcional por comando (0 = sin timeout adicional al del ctx).
	Timeout time.Duration
}

// Run ejecuta el comando y replica la semántica histórica de compose.run /
// services.runDocker: CombinedOutput + TrimSpace + mensaje de error desde la salida.
func (c Cmd) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	slog.Debug("runner: exec", "dir", dir, "cmd", name, "args", args)

	start := time.Now()
	output, err := cmd.CombinedOutput()
	slog.Debug("runner: done", "cmd", name, "elapsed", time.Since(start).String(), "err", err)

	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", err
		}

		return "", fmt.Errorf("%s", trimmed)
	}

	return trimmed, nil
}
