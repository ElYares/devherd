package services

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/runner"
)

// composeContent es el manifiesto de los servicios compartidos (redis, mailpit).
// Se embebe desde un .yml real para que editores y linters de YAML lo validen,
// igual que database/schema.sql.
//
//go:embed shared-services.compose.yml
var composeContent string

const (
	NetworkName = "infra_net"
	stackDir    = "shared-services"
	composeFile = "docker-compose.yml"
)

var supportedServices = []string{"redis", "mailpit"}

type Manager struct {
	dir         string
	composeFile string
	run         runner.Runner
}

func NewManager(paths config.Paths) Manager {
	return NewManagerWithRunner(paths, runner.Cmd{})
}

// NewManagerWithRunner permite inyectar un Runner (útil para tests sin Docker).
func NewManagerWithRunner(paths config.Paths, r runner.Runner) Manager {
	dir := filepath.Join(paths.ComposeDir, stackDir)
	return Manager{
		dir:         dir,
		composeFile: filepath.Join(dir, composeFile),
		run:         r,
	}
}

func SupportedServices() []string {
	return append([]string{}, supportedServices...)
}

func (m Manager) Start(ctx context.Context, service string) (string, error) {
	if err := validateService(service); err != nil {
		return "", err
	}

	if err := m.bootstrap(); err != nil {
		return "", err
	}

	if err := m.ensureNetwork(ctx); err != nil {
		return "", err
	}

	return m.compose(ctx, "up", "-d", service)
}

func (m Manager) Stop(ctx context.Context, service string) (string, error) {
	if err := validateService(service); err != nil {
		return "", err
	}

	if err := m.bootstrap(); err != nil {
		return "", err
	}

	return m.compose(ctx, "stop", service)
}

func (m Manager) Status(ctx context.Context, service string) (string, error) {
	if service != "" {
		if err := validateService(service); err != nil {
			return "", err
		}
	}

	if err := m.bootstrap(); err != nil {
		return "", err
	}

	args := []string{"ps"}
	if service != "" {
		args = append(args, service)
	}

	return m.compose(ctx, args...)
}

func (m Manager) bootstrap() error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create shared services directory: %w", err)
	}

	if err := os.WriteFile(m.composeFile, []byte(composeContent), 0o644); err != nil {
		return fmt.Errorf("write shared services compose: %w", err)
	}

	return nil
}

func (m Manager) compose(ctx context.Context, args ...string) (string, error) {
	baseArgs := []string{"compose", "-f", m.composeFile, "--project-name", "devherd_shared"}
	baseArgs = append(baseArgs, args...)

	return m.run.Run(ctx, m.dir, "docker", baseArgs...)
}

func validateService(service string) error {
	if slices.Contains(supportedServices, service) {
		return nil
	}

	return fmt.Errorf("unsupported shared service %q; supported services: %s", service, strings.Join(supportedServices, ", "))
}

func (m Manager) ensureNetwork(ctx context.Context) error {
	if _, err := m.run.Run(ctx, "", "docker", "network", "inspect", NetworkName); err == nil {
		return nil
	}

	if _, err := m.run.Run(
		ctx,
		"",
		"docker",
		"network",
		"create",
		"--driver",
		"bridge",
		"--label",
		"devherd.managed=true",
		"--label",
		"devherd.role=shared-services",
		NetworkName,
	); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}

		return fmt.Errorf("ensure docker network %s: %w", NetworkName, err)
	}

	return nil
}

