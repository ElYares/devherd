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
// igual que las migraciones en database/migrations/.
//
//go:embed shared-services.compose.yml
var composeContent string

const (
	NetworkName = "infra_net"
	stackDir    = "shared-services"
	composeFile = "docker-compose.yml"
)

// supportedServices es el catalogo. Prometheus es opcional como los demas: nada
// del producto depende de que este arrancado.
var supportedServices = []string{"redis", "mailpit", "prometheus", "grafana"}

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

// StartOptions son las decisiones de quien arranca el servicio. Van en una
// estructura y no como parametros sueltos porque ya son dos y creceran con cada
// servicio que necesite algo del entorno.
type StartOptions struct {
	// Force devuelve los archivos de configuracion administrados a la plantilla
	// de DevHerd, guardando antes una copia.
	Force bool
	// CollectorAddr es la direccion del collector de Observe alcanzable desde un
	// contenedor. Solo la usan los servicios que la piden; ver NeedsCollector.
	CollectorAddr string
}

// Start levanta un servicio compartido.
func (m Manager) Start(ctx context.Context, service string, opts StartOptions) (string, []FileResult, error) {
	if err := validateService(service); err != nil {
		return "", nil, err
	}

	if err := m.bootstrap(); err != nil {
		return "", nil, err
	}

	files, err := m.ensureServiceFiles(service, opts)
	if err != nil {
		return "", nil, err
	}

	if err := m.ensureNetwork(ctx); err != nil {
		return "", files, err
	}

	output, err := m.compose(ctx, "up", "-d", service)

	return output, files, err
}

// DependsOn dice que otro servicio compartido necesita este para servir de algo.
// Grafana sin Prometheus arranca perfectamente y muestra paneles vacios, que es la
// peor forma de fallar: parece que funciona.
func DependsOn(service string) string {
	if service == "grafana" {
		return "prometheus"
	}

	return ""
}

// IsRunning dice si un servicio compartido esta levantado. Se consulta a docker y
// no al disco: que el compose lo declare no significa que el contenedor viva.
func (m Manager) IsRunning(ctx context.Context, service string) (bool, error) {
	if err := validateService(service); err != nil {
		return false, err
	}

	exists, err := m.stackExists()
	if err != nil || !exists {
		return false, err
	}

	out, err := m.compose(ctx, "ps", "--status", "running", "--services")
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == service {
			return true, nil
		}
	}

	return false, nil
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

// Status consulta el estado y **no escribe nada**. Antes llamaba a bootstrap, asi
// que un comando de consulta reescribia el compose en disco. No se notaba porque
// el contenido era identico, pero es un efecto que nadie pidio y que rompe en
// cuanto un servicio tiene configuracion editable.
func (m Manager) Status(ctx context.Context, service string) (string, error) {
	if service != "" {
		if err := validateService(service); err != nil {
			return "", err
		}
	}

	exists, err := m.stackExists()
	if err != nil {
		return "", err
	}
	if !exists {
		// Nunca se arranco nada. La respuesta honesta a "que esta corriendo" es
		// "nada", no un error, y desde luego no crear el compose para que
		// `docker compose ps` devuelva una tabla vacia.
		return "no shared services have been started yet; run `devherd service start <service>` (" +
			strings.Join(supportedServices, ", ") + ")", nil
	}

	args := []string{"ps"}
	if service != "" {
		args = append(args, service)
	}

	return m.compose(ctx, args...)
}

// bootstrap prepara el stack para una operacion que va a cambiar algo. El compose
// se regenera siempre a proposito: es el catalogo de lo que DevHerd ofrece, no la
// configuracion del usuario. Congelarlo con la edicion de alguien haria que una
// version nueva del binario no pudiera ofrecer un servicio nuevo.
func (m Manager) bootstrap() error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create shared services directory: %w", err)
	}

	if err := os.WriteFile(m.composeFile, []byte(composeContent), 0o644); err != nil {
		return fmt.Errorf("write shared services compose: %w", err)
	}

	return nil
}

// stackExists dice si el compose ya esta escrito, sin escribirlo. Es lo unico que
// necesita saber una operacion de solo lectura.
func (m Manager) stackExists() (bool, error) {
	switch _, err := os.Stat(m.composeFile); {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("read shared services compose: %w", err)
	}
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
