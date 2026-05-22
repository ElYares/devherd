package preflight

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/proxy"
	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityOK   Severity = "ok"
	SeverityWarn Severity = "warn"
	SeverityFail Severity = "fail"
)

type Finding struct {
	Severity Severity
	Name     string
	Message  string
}

type Report struct {
	Project  compose.Project
	Findings []Finding
}

func (r Report) HasFailures() bool {
	return r.Count(SeverityFail) > 0
}

func (r Report) HasWarnings() bool {
	return r.Count(SeverityWarn) > 0
}

func (r Report) Count(severity Severity) int {
	count := 0
	for _, finding := range r.Findings {
		if finding.Severity == severity {
			count++
		}
	}

	return count
}

type composeDoc struct {
	Services map[string]serviceConfig `yaml:"services"`
	Volumes  map[string]volumeConfig  `yaml:"volumes"`
}

type serviceConfig struct {
	ContainerName string `yaml:"container_name"`
	Ports         []any  `yaml:"ports"`
	Networks      any    `yaml:"networks"`
	EnvFile       any    `yaml:"env_file"`
	Volumes       []any  `yaml:"volumes"`
	Environment   any    `yaml:"environment"`
}

type volumeConfig struct {
	External any    `yaml:"external"`
	Name     string `yaml:"name"`
}

type servicePort struct {
	Service string
	Host    string
	Port    int
	Raw     string
}

type dockerContainer struct {
	Name    string
	Status  string
	Project string
	Ports   string
}

func Inspect(ctx context.Context, targetPath string, cfg config.Config) (Report, error) {
	project, err := compose.ResolveProject(targetPath)
	if err != nil {
		return Report{}, err
	}

	cfg.ApplyPathDefaults(mustResolvePaths())

	report := Report{Project: project}
	env := readProjectEnv(project)
	docs, err := readComposeDocs(project.ComposeFiles)
	if err != nil {
		return Report{}, err
	}

	containers := dockerContainers(ctx)
	composeProjectName := project.ProjectName

	report.add(SeverityOK, "project", fmt.Sprintf("root %s", project.Root))
	report.add(SeverityOK, "compose", fmt.Sprintf("%d file(s) resolved", len(project.ComposeFiles)))

	report.Findings = append(report.Findings, inspectContainerNames(docs, env, containers, composeProjectName)...)
	report.Findings = append(report.Findings, inspectPorts(docs, env, containers, composeProjectName)...)
	report.Findings = append(report.Findings, inspectVolumes(docs)...)
	report.Findings = append(report.Findings, inspectLaravelEnv(project, env)...)
	report.Findings = append(report.Findings, inspectSharedNetworks(docs, env)...)
	report.Findings = append(report.Findings, inspectExternalProxy(ctx, project, cfg)...)

	sort.SliceStable(report.Findings, func(i, j int) bool {
		return severityRank(report.Findings[i].Severity) > severityRank(report.Findings[j].Severity)
	})

	return report, nil
}

func (r *Report) add(severity Severity, name, message string) {
	r.Findings = append(r.Findings, Finding{
		Severity: severity,
		Name:     name,
		Message:  message,
	})
}

func readComposeDocs(files []string) ([]composeDoc, error) {
	docs := make([]composeDoc, 0, len(files))
	for _, file := range files {
		payload, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read compose file %s: %w", file, err)
		}

		var doc composeDoc
		if err := yaml.Unmarshal(payload, &doc); err != nil {
			return nil, fmt.Errorf("parse compose file %s: %w", file, err)
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func readProjectEnv(project compose.Project) map[string]string {
	envPath := project.EnvFile
	if envPath == "" {
		envPath = filepath.Join(project.Root, ".env")
	}

	return readEnvFile(envPath)
}

func readEnvFile(path string) map[string]string {
	values := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			values[key] = value
		}
	}

	return values
}

func inspectContainerNames(docs []composeDoc, env map[string]string, containers map[string]dockerContainer, composeProjectName string) []Finding {
	var findings []Finding
	for _, service := range collectServices(docs) {
		if service.Config.ContainerName == "" {
			continue
		}

		resolvedName := expandEnv(service.Config.ContainerName, env)
		usesPrefix := strings.Contains(service.Config.ContainerName, "COMPOSE_NAME_PREFIX")
		if usesPrefix && env["COMPOSE_NAME_PREFIX"] == "" {
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "container_name",
				Message:  fmt.Sprintf("%s parameterizes container_name but COMPOSE_NAME_PREFIX is not set in .env", service.Name),
			})
			continue
		}

		existing, exists := containers[resolvedName]
		switch {
		case exists && existing.Project != "" && existing.Project != composeProjectName:
			findings = append(findings, Finding{
				Severity: SeverityFail,
				Name:     "container_name",
				Message:  fmt.Sprintf("%s uses %q but it belongs to compose project %q", service.Name, resolvedName, existing.Project),
			})
		case usesPrefix:
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Name:     "container_name",
				Message:  fmt.Sprintf("%s uses parameterized name %q", service.Name, resolvedName),
			})
		case exists:
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "container_name",
				Message:  fmt.Sprintf("%s pins %q; clones or parallel variants of this project will collide", service.Name, resolvedName),
			})
		default:
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "container_name",
				Message:  fmt.Sprintf("%s pins %q; this reduces Compose isolation", service.Name, resolvedName),
			})
		}
	}

	return findings
}

func inspectPorts(docs []composeDoc, env map[string]string, containers map[string]dockerContainer, composeProjectName string) []Finding {
	var findings []Finding
	for _, port := range collectPorts(docs, env) {
		owner, owned := portOwner(port.Port, containers)
		if !isPortListening(port.Port) {
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Name:     "port",
				Message:  fmt.Sprintf("%s publishes %s%d and it is available", port.Service, hostPrefix(port.Host), port.Port),
			})
			continue
		}

		if owned && owner.Project == composeProjectName {
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Name:     "port",
				Message:  fmt.Sprintf("%s publishes %s%d and it is already owned by this project", port.Service, hostPrefix(port.Host), port.Port),
			})
			continue
		}

		message := fmt.Sprintf("%s wants %s%d but the port is already in use", port.Service, hostPrefix(port.Host), port.Port)
		if owned {
			message = fmt.Sprintf("%s wants %s%d but %s owns it", port.Service, hostPrefix(port.Host), port.Port, owner.Name)
		}
		findings = append(findings, Finding{Severity: SeverityFail, Name: "port", Message: message})
	}

	return findings
}

func inspectVolumes(docs []composeDoc) []Finding {
	var findings []Finding
	for _, doc := range docs {
		for name, volume := range doc.Volumes {
			if isExternal(volume.External) {
				message := fmt.Sprintf("volume %q is external", name)
				if volume.Name != "" {
					message = fmt.Sprintf("volume %q uses external name %q", name, volume.Name)
				}
				findings = append(findings, Finding{Severity: SeverityWarn, Name: "volume", Message: message})
			}
		}
	}

	return findings
}

func inspectLaravelEnv(project compose.Project, env map[string]string) []Finding {
	if len(env) == 0 {
		return []Finding{{Severity: SeverityWarn, Name: "env", Message: ".env was not found or is empty"}}
	}

	var findings []Finding
	if project.Proxy.Domain != "" && env["APP_URL"] != "" && !strings.Contains(env["APP_URL"], project.Proxy.Domain) {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Name:     "APP_URL",
			Message:  fmt.Sprintf("APP_URL is %q but proxy domain is %q", env["APP_URL"], project.Proxy.Domain),
		})
	}

	if _, ok := env["SESSION_COOKIE"]; !ok {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Name:     "session",
			Message:  "SESSION_COOKIE is not set; Laravel defaults can collide across local apps",
		})
	}

	if usesRedis(env) {
		if env["REDIS_PREFIX"] == "" {
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "redis",
				Message:  "Redis is used but REDIS_PREFIX is not set",
			})
		}
		if env["REDIS_DB"] == "" || env["REDIS_CACHE_DB"] == "" {
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "redis",
				Message:  "Redis is used but REDIS_DB/REDIS_CACHE_DB are not both set",
			})
		}
		if env["REDIS_DB"] != "" && env["REDIS_DB"] == env["REDIS_CACHE_DB"] {
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Name:     "redis",
				Message:  "REDIS_DB and REDIS_CACHE_DB point to the same logical database",
			})
		}
	}

	if strings.EqualFold(env["CACHE_STORE"], "redis") && env["CACHE_PREFIX"] == "" {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Name:     "cache",
			Message:  "CACHE_STORE=redis but CACHE_PREFIX is not set",
		})
	}

	return findings
}

func inspectSharedNetworks(docs []composeDoc, env map[string]string) []Finding {
	if !usesRedis(env) {
		return nil
	}

	for _, service := range collectServices(docs) {
		if serviceUsesNetwork(service.Config.Networks, "infra_net") {
			return []Finding{{
				Severity: SeverityWarn,
				Name:     "shared-service",
				Message:  "project can reach shared Redis on infra_net; namespace Redis keys per project",
			}}
		}
	}

	return nil
}

func inspectExternalProxy(ctx context.Context, project compose.Project, cfg config.Config) []Finding {
	if !proxy.UsesDockerExternal(cfg) || project.Proxy.Domain == "" {
		return nil
	}

	caddyfile := filepath.Join(cfg.Proxy.ExternalDir, proxy.ExternalProxyCaddyfile)
	payload, err := os.ReadFile(caddyfile)
	if err != nil {
		return []Finding{{Severity: SeverityWarn, Name: "proxy", Message: fmt.Sprintf("could not read external Caddyfile: %s", err.Error())}}
	}

	hasBlock := strings.Contains(string(payload), "http://"+project.Proxy.Domain+" {") || strings.Contains(string(payload), project.Proxy.Domain+" {")
	serviceRunning := composeServiceRunning(ctx, project, project.Proxy.Service)

	switch {
	case hasBlock && serviceRunning:
		return []Finding{{Severity: SeverityOK, Name: "proxy", Message: fmt.Sprintf("%s is published and %s is running", project.Proxy.Domain, project.Proxy.Service)}}
	case hasBlock:
		return []Finding{{Severity: SeverityWarn, Name: "proxy", Message: fmt.Sprintf("%s is still in Caddyfile but service %s is not running", project.Proxy.Domain, project.Proxy.Service)}}
	case serviceRunning:
		return []Finding{{Severity: SeverityWarn, Name: "proxy", Message: fmt.Sprintf("service %s is running but %s is not in Caddyfile; run proxy apply", project.Proxy.Service, project.Proxy.Domain)}}
	default:
		return []Finding{{Severity: SeverityOK, Name: "proxy", Message: fmt.Sprintf("%s is not published and service %s is not running", project.Proxy.Domain, project.Proxy.Service)}}
	}
}

type namedService struct {
	Name   string
	Config serviceConfig
}

func collectServices(docs []composeDoc) []namedService {
	var services []namedService
	for _, doc := range docs {
		for name, service := range doc.Services {
			services = append(services, namedService{Name: name, Config: service})
		}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services
}

func collectPorts(docs []composeDoc, env map[string]string) []servicePort {
	var ports []servicePort
	for _, service := range collectServices(docs) {
		for _, raw := range service.Config.Ports {
			port, ok := parsePort(service.Name, raw, env)
			if ok {
				ports = append(ports, port)
			}
		}
	}
	return ports
}

func parsePort(service string, raw any, env map[string]string) (servicePort, bool) {
	switch value := raw.(type) {
	case string:
		resolved := expandEnv(value, env)
		parts := strings.Split(resolved, ":")
		if len(parts) < 2 {
			return servicePort{}, false
		}

		host := ""
		hostPort := parts[0]
		if len(parts) >= 3 {
			host = strings.Join(parts[:len(parts)-2], ":")
			hostPort = parts[len(parts)-2]
		}
		port, err := strconv.Atoi(strings.Trim(hostPort, `"'`))
		if err != nil {
			return servicePort{}, false
		}
		return servicePort{Service: service, Host: host, Port: port, Raw: resolved}, true
	case map[string]any:
		published, ok := value["published"]
		if !ok {
			return servicePort{}, false
		}
		port, err := strconv.Atoi(fmt.Sprint(published))
		if err != nil {
			return servicePort{}, false
		}
		host := fmt.Sprint(value["host_ip"])
		return servicePort{Service: service, Host: host, Port: port, Raw: fmt.Sprint(raw)}, true
	default:
		return servicePort{}, false
	}
}

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

func expandEnv(value string, env map[string]string) string {
	return envPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := envPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		if val, ok := env[parts[1]]; ok && val != "" {
			return val
		}
		return parts[2]
	})
}

func isPortListening(port int) bool {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err == nil {
		_ = listener.Close()
		return false
	}

	return true
}

func dockerContainers(ctx context.Context) map[string]dockerContainer {
	output, err := run(ctx, "docker", "ps", "-a", "--format", "{{.Names}}\t{{.Status}}\t{{.Label \"com.docker.compose.project\"}}\t{{.Ports}}")
	if err != nil {
		return map[string]dockerContainer{}
	}

	containers := map[string]dockerContainer{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) < 4 {
			continue
		}
		containers[fields[0]] = dockerContainer{
			Name:    fields[0],
			Status:  fields[1],
			Project: fields[2],
			Ports:   fields[3],
		}
	}

	return containers
}

func portOwner(port int, containers map[string]dockerContainer) (dockerContainer, bool) {
	needle := strconv.Itoa(port) + "->"
	for _, container := range containers {
		if strings.Contains(container.Ports, needle) {
			return container, true
		}
	}
	return dockerContainer{}, false
}

func composeServiceRunning(ctx context.Context, project compose.Project, service string) bool {
	if service == "" {
		return false
	}

	args := append(compose.Command(project), "ps", "-q", service)
	output, err := run(ctx, args[0], args[1:]...)
	return err == nil && strings.TrimSpace(output) != ""
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, name, args...)
	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func hostPrefix(host string) string {
	if host == "" {
		return ""
	}
	return host + ":"
}

func usesRedis(env map[string]string) bool {
	for _, key := range []string{"CACHE_STORE", "QUEUE_CONNECTION", "SESSION_DRIVER"} {
		if strings.EqualFold(env[key], "redis") {
			return true
		}
	}
	return strings.EqualFold(env["REDIS_HOST"], "redis") || strings.EqualFold(env["REDIS_HOST"], "127.0.0.1")
}

func serviceUsesNetwork(networks any, name string) bool {
	switch value := networks.(type) {
	case []any:
		for _, item := range value {
			if fmt.Sprint(item) == name {
				return true
			}
		}
	case map[string]any:
		_, ok := value[name]
		return ok
	}
	return false
}

func isExternal(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case map[string]any:
		if flag, ok := typed["external"].(bool); ok {
			return flag
		}
	}
	return false
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityFail:
		return 3
	case SeverityWarn:
		return 2
	default:
		return 1
	}
}

func mustResolvePaths() config.Paths {
	paths, err := config.ResolvePaths()
	if err != nil {
		return config.Paths{}
	}
	return paths
}
