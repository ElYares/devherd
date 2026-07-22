package observe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// UnitName es la unidad de usuario que mantiene vivo el collector. Se instala
// bajo systemd --user y no como servicio de sistema: el collector escribe en la
// base del usuario y no necesita privilegios.
const UnitName = "devherd-observe.service"

const unitTemplate = `[Unit]
Description=DevHerd Observe collector
Documentation=https://github.com/devherd/devherd
After=docker.service

[Service]
Type=simple
ExecStart={{.Binary}} observe start{{if .Addr}} --addr {{.Addr}}{{end}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

type unitData struct {
	Binary string
	Addr   string
}

// UnitPath devuelve la ruta de la unidad de usuario segun XDG.
func UnitPath() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("systemd user services are only available on Linux")
	}

	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}

	return filepath.Join(configHome, "systemd", "user", UnitName), nil
}

// RenderUnit genera la unidad. addr vacio deja que el collector resuelva sus
// direcciones por defecto (loopback + gateways de las redes DevHerd).
func RenderUnit(binary, addr string) (string, error) {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		return "", fmt.Errorf("binary path is required")
	}

	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, unitData{Binary: binary, Addr: strings.TrimSpace(addr)}); err != nil {
		return "", err
	}

	return out.String(), nil
}

// InstallDaemon escribe la unidad y la habilita. Devuelve la ruta escrita.
func InstallDaemon(ctx context.Context, binary, addr string) (string, error) {
	path, err := UnitPath()
	if err != nil {
		return "", err
	}

	content, err := RenderUnit(binary, addr)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create systemd user directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write systemd unit: %w", err)
	}

	if err := systemctl(ctx, "daemon-reload"); err != nil {
		return path, err
	}
	if err := systemctl(ctx, "enable", "--now", UnitName); err != nil {
		return path, err
	}

	return path, nil
}

// UninstallDaemon para la unidad y borra el archivo.
func UninstallDaemon(ctx context.Context) (string, error) {
	path, err := UnitPath()
	if err != nil {
		return "", err
	}

	// Se ignoran los errores de parada: la unidad puede no estar cargada y aun
	// asi queremos borrar el archivo.
	_ = systemctl(ctx, "disable", "--now", UnitName)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return path, fmt.Errorf("remove systemd unit: %w", err)
	}

	_ = systemctl(ctx, "daemon-reload")

	return path, nil
}

// DaemonStatus devuelve la salida de `systemctl --user status`.
func DaemonStatus(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "--no-pager", "status", UnitName)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil && trimmed == "" {
		return "", err
	}

	return trimmed, nil
}

func systemctl(ctx context.Context, args ...string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is not available on this host")
	}

	full := append([]string{"--user"}, args...)
	cmd := exec.CommandContext(ctx, "systemctl", full...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), trimmed)
		}

		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}

	return nil
}
