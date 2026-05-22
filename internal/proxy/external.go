package proxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
)

const (
	DriverCaddy               = "caddy"
	DriverNginx               = "nginx"
	DriverCaddyDockerExternal = "caddy-docker-external"

	ExternalProxyCaddyfile     = "Caddyfile"
	ExternalProxyComposeFile   = "docker-compose.yml"
	ExternalProxyEnvFile       = ".env"
	ManagedComposeOverrideFile = ".devherd.proxy.override.yml"
)

type Alias struct {
	Service string
	Name    string
}

type ExternalProject struct {
	Project database.ProjectRecord
	Compose compose.Project
	Domain  string
	Routes  []Route
	Aliases []Alias
}

type externalSettingsConfig struct {
	Dir           string
	Network       string
	ContainerName string
}

func UsesDockerExternal(cfg config.Config) bool {
	return cfg.Proxy.Driver == DriverCaddyDockerExternal
}

func externalSettings(cfg config.Config) externalSettingsConfig {
	settings := externalSettingsConfig{
		Dir:           cfg.Proxy.ExternalDir,
		Network:       cfg.Proxy.ExternalNetwork,
		ContainerName: cfg.Proxy.ExternalContainerName,
	}

	if settings.Dir == "" {
		settings.Dir = config.Default().Proxy.ExternalDir
	}
	if settings.Network == "" {
		settings.Network = "infra_web"
	}
	if settings.ContainerName == "" {
		settings.ContainerName = "infra_caddy"
	}

	return settings
}

func DefaultTLDForDriver(driver string) string {
	if driver == DriverCaddyDockerExternal {
		return "localhost"
	}

	return "test"
}

func ProjectDomain(cfg config.Config, project database.ProjectRecord) (string, error) {
	composeProject, err := compose.ResolveProject(project.Path)
	if err != nil {
		return "", err
	}

	return effectiveDomain(cfg, project, composeProject), nil
}

func BuildExternalProject(cfg config.Config, project database.ProjectRecord) (ExternalProject, error) {
	composeProject, err := compose.ResolveProject(project.Path)
	if err != nil {
		return ExternalProject{}, err
	}

	domain := effectiveDomain(cfg, project, composeProject)
	prefix := domainPrefix(domain)

	if composeProject.Proxy.Service != "" && composeProject.Proxy.Port > 0 {
		alias := aliasName(prefix, composeProject.Proxy.Service)
		return ExternalProject{
			Project: project,
			Compose: composeProject,
			Domain:  domain,
			Routes: []Route{
				{Matcher: "/*", Target: fmt.Sprintf("%s:%d", alias, composeProject.Proxy.Port)},
			},
			Aliases: []Alias{{Service: composeProject.Proxy.Service, Name: alias}},
		}, nil
	}

	switch project.Framework {
	case "vue+flask":
		return ExternalProject{
			Project: project,
			Compose: composeProject,
			Domain:  domain,
			Routes: []Route{
				{Matcher: "/api/*", Target: fmt.Sprintf("%s:8000", aliasName(prefix, "backend"))},
				{Matcher: "/*", Target: fmt.Sprintf("%s:5173", aliasName(prefix, "frontend"))},
			},
			Aliases: []Alias{
				{Service: "backend", Name: aliasName(prefix, "backend")},
				{Service: "frontend", Name: aliasName(prefix, "frontend")},
			},
		}, nil
	default:
		return ExternalProject{}, fmt.Errorf("project %q needs manifest proxy metadata or a supported framework for external proxy mode", project.Name)
	}
}

func EnsureComposeOverride(cfg config.Config, project ExternalProject) (string, error) {
	settings := externalSettings(cfg)

	if len(project.Aliases) == 0 {
		return "", errors.New("external proxy project has no service aliases to manage")
	}

	var builder strings.Builder
	builder.WriteString("services:\n")
	for _, alias := range project.Aliases {
		builder.WriteString("  ")
		builder.WriteString(alias.Service)
		builder.WriteString(":\n")
		builder.WriteString("    networks:\n")
		builder.WriteString("      ")
		builder.WriteString(settings.Network)
		builder.WriteString(":\n")
		builder.WriteString("        aliases:\n")
		builder.WriteString("          - ")
		builder.WriteString(alias.Name)
		builder.WriteString("\n")
	}
	builder.WriteString("\nnetworks:\n")
	builder.WriteString("  ")
	builder.WriteString(settings.Network)
	builder.WriteString(":\n")
	builder.WriteString("    external: true\n")

	overridePath := filepath.Join(project.Compose.Root, ManagedComposeOverrideFile)
	if err := os.WriteFile(overridePath, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write managed compose override: %w", err)
	}

	return overridePath, nil
}

func ConnectProject(ctx context.Context, cfg config.Config, project ExternalProject) error {
	settings := externalSettings(cfg)

	if err := ensureExternalProxyNetwork(ctx, settings); err != nil {
		return err
	}

	for _, alias := range project.Aliases {
		containerName, err := composeServiceContainer(ctx, project.Compose, alias.Service)
		if err != nil {
			continue
		}

		if _, err := runCommand(ctx, "", "docker", "network", "connect", "--alias", alias.Name, settings.Network, containerName); err != nil {
			message := err.Error()
			if strings.Contains(message, "already exists") || strings.Contains(message, "already connected") {
				continue
			}

			return fmt.Errorf("connect %s to %s with alias %s: %w", containerName, settings.Network, alias.Name, err)
		}
	}

	return nil
}

func ApplyExternalProxy(ctx context.Context, cfg config.Config, projects []ExternalProject) (string, []string, error) {
	settings := externalSettings(cfg)

	if _, err := BootstrapExternalProxy(cfg); err != nil {
		return "", nil, err
	}

	configPath := filepath.Join(settings.Dir, ExternalProxyCaddyfile)
	content, domains, err := mergeExternalProxyConfig(configPath, projects)
	if err != nil {
		return "", nil, err
	}

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", nil, fmt.Errorf("write local_proxy Caddyfile: %w", err)
	}

	if err := ensureExternalProxyReady(ctx, settings); err != nil {
		return "", nil, err
	}

	containerName := externalProxyContainerName(cfg)
	if _, err := runCommand(ctx, "", "docker", "exec", containerName, "caddy", "validate", "--config", "/etc/caddy/Caddyfile"); err != nil {
		return "", nil, fmt.Errorf("validate local_proxy Caddyfile: %w", err)
	}

	if _, err := runCommand(ctx, "", "docker", "exec", containerName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile"); err != nil {
		return "", nil, fmt.Errorf("reload local_proxy Caddyfile: %w", err)
	}

	return configPath, domains, nil
}

func RemoveExternalProxy(ctx context.Context, cfg config.Config, domains []string) (string, error) {
	if len(domains) == 0 {
		return "", nil
	}

	settings := externalSettings(cfg)
	if _, err := BootstrapExternalProxy(cfg); err != nil {
		return "", err
	}

	configPath := filepath.Join(settings.Dir, ExternalProxyCaddyfile)
	existing, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read local_proxy Caddyfile: %w", err)
	}

	content := stripManagedDomains(string(existing), domains)
	content = strings.TrimRight(content, "\n") + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write local_proxy Caddyfile: %w", err)
	}

	if err := ensureExternalProxyReady(ctx, settings); err != nil {
		return "", err
	}

	containerName := externalProxyContainerName(cfg)
	if _, err := runCommand(ctx, "", "docker", "exec", containerName, "caddy", "validate", "--config", "/etc/caddy/Caddyfile"); err != nil {
		return "", fmt.Errorf("validate local_proxy Caddyfile: %w", err)
	}

	if _, err := runCommand(ctx, "", "docker", "exec", containerName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile"); err != nil {
		return "", fmt.Errorf("reload local_proxy Caddyfile: %w", err)
	}

	return configPath, nil
}

func effectiveDomain(cfg config.Config, project database.ProjectRecord, composeProject compose.Project) string {
	if composeProject.Proxy.Domain != "" {
		return composeProject.Proxy.Domain
	}

	if project.Domain != "" {
		return project.Domain
	}

	return project.Name + "." + strings.TrimPrefix(cfg.LocalTLD, ".")
}

func domainPrefix(domain string) string {
	parts := strings.Split(strings.TrimSpace(domain), ".")
	if len(parts) <= 1 {
		return primaryLabel(domain)
	}

	return primaryLabel(strings.Join(parts[:len(parts)-1], "-"))
}

func aliasName(prefix, service string) string {
	return primaryLabel(prefix) + "-" + primaryLabel(service)
}

func primaryLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			builder.WriteRune('-')
			lastHyphen = true
		}
	}

	label := strings.Trim(builder.String(), "-")
	if label == "" {
		return "project"
	}

	return label
}

func mergeExternalProxyConfig(path string, projects []ExternalProject) (string, []string, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read local_proxy Caddyfile: %w", err)
	}

	domains := make([]string, 0, len(projects))
	for _, project := range projects {
		domains = append(domains, project.Domain)
	}

	base := stripManagedDomains(string(existing), domains)
	rendered := renderExternalSites(projects)
	if strings.TrimSpace(base) == "" {
		return rendered, domains, nil
	}

	return strings.TrimRight(base, "\n") + "\n\n" + rendered, domains, nil
}

func renderExternalSites(projects []ExternalProject) string {
	var blocks []string
	for _, project := range projects {
		blocks = append(blocks, renderExternalSite(project))
	}

	return strings.Join(blocks, "\n\n")
}

func renderExternalSite(project ExternalProject) string {
	var builder strings.Builder
	builder.WriteString("http://")
	builder.WriteString(project.Domain)
	builder.WriteString(" {\n")
	for _, route := range project.Routes {
		if route.Matcher == "/*" {
			builder.WriteString("\thandle {\n")
			builder.WriteString("\t\treverse_proxy ")
			builder.WriteString(route.Target)
			builder.WriteString("\n\t}\n")
			continue
		}

		builder.WriteString("\thandle ")
		builder.WriteString(route.Matcher)
		builder.WriteString(" {\n")
		builder.WriteString("\t\treverse_proxy ")
		builder.WriteString(route.Target)
		builder.WriteString("\n\t}\n")
	}
	builder.WriteString("}")
	return builder.String()
}

func stripManagedDomains(content string, domains []string) string {
	lines := strings.Split(content, "\n")
	domainHeaders := make(map[string]struct{}, len(domains)*2)
	for _, domain := range domains {
		domainHeaders["http://"+domain+" {"] = struct{}{}
		domainHeaders[domain+" {"] = struct{}{}
	}

	var output []string
	skipping := false
	depth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !skipping {
			if _, ok := domainHeaders[trimmed]; ok {
				skipping = true
				depth = strings.Count(line, "{") - strings.Count(line, "}")
				if depth <= 0 {
					skipping = false
				}
				continue
			}

			output = append(output, line)
			continue
		}

		depth += strings.Count(line, "{")
		depth -= strings.Count(line, "}")
		if depth <= 0 {
			skipping = false
		}
	}

	return strings.TrimRight(strings.Join(output, "\n"), "\n")
}

func composeServiceContainer(ctx context.Context, project compose.Project, service string) (string, error) {
	args := append(compose.Command(project), "ps", "-q", service)
	output, err := runCommand(ctx, project.Root, args[0], args[1:]...)
	if err != nil {
		return "", err
	}

	containerID := strings.TrimSpace(output)
	if containerID == "" {
		return "", errors.New("service is not running")
	}

	name, err := runCommand(ctx, "", "docker", "inspect", "--format", "{{.Name}}", containerID)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(strings.TrimSpace(name), "/"), nil
}

func ensureExternalProxyReady(ctx context.Context, settings externalSettingsConfig) error {
	if err := ensureExternalProxyNetwork(ctx, settings); err != nil {
		return err
	}

	if _, err := runCommand(ctx, settings.Dir, "docker", "compose", "up", "-d"); err != nil {
		return fmt.Errorf("start local_proxy: %w", err)
	}

	return nil
}

func ensureExternalProxyNetwork(ctx context.Context, settings externalSettingsConfig) error {
	if _, err := runCommand(ctx, "", "docker", "network", "inspect", settings.Network); err != nil {
		if _, createErr := runCommand(
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
			"devherd.role=shared-proxy",
			settings.Network,
		); createErr != nil {
			if strings.Contains(createErr.Error(), "already exists") {
				return nil
			}

			return fmt.Errorf("ensure docker network %s: %w", settings.Network, createErr)
		}
	}

	return nil
}

func externalProxyContainerName(cfg config.Config) string {
	return externalSettings(cfg).ContainerName
}

func runCommand(ctx context.Context, workdir string, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, name, args...)
	if workdir != "" {
		if info, err := os.Stat(workdir); err == nil && info.IsDir() {
			cmd.Dir = workdir
		} else if filepath.Ext(workdir) != "" {
			cmd.Dir = filepath.Dir(workdir)
		}
	}

	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out")
	}

	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", err
		}

		return "", errors.New(firstLine(message))
	}

	return strings.TrimSpace(string(output)), nil
}

func firstLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "ok"
	}

	return lines[0]
}

func URLForDomain(cfg config.Config, domain string) string {
	if cfg.Proxy.HTTPPort == 80 {
		return "http://" + domain
	}

	return "http://" + domain + ":" + strconv.Itoa(cfg.Proxy.HTTPPort)
}
