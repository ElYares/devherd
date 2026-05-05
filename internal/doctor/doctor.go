package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/config"
)

type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

type Check struct {
	Name    string
	Status  Status
	Message string
}

type Report struct {
	Checks []Check
}

type dockerEngineInfo struct {
	OSType          string
	OperatingSystem string
	Name            string
}

type dockerNetworkInfo struct {
	Driver   string
	Scope    string
	Internal bool
}

func Run(ctx context.Context) Report {
	return RunWithConfig(ctx, config.Default())
}

func RunWithConfig(ctx context.Context, cfg config.Config) Report {
	paths, err := config.ResolvePaths()
	if err == nil {
		cfg.ApplyPathDefaults(paths)
	}

	checks := []Check{
		checkLocalPaths(),
		checkBinary("docker", "Docker CLI"),
		checkDockerDaemon(ctx),
		checkDockerEngineMode(ctx),
		checkDockerCompose(ctx),
	}

	if cfg.Proxy.Driver == "caddy-docker-external" {
		checks = append(checks,
			checkDirectory(cfg.Proxy.ExternalDir, "local_proxy dir"),
			checkFile(filepath.Join(cfg.Proxy.ExternalDir, "docker-compose.yml"), "local_proxy compose"),
			checkFile(filepath.Join(cfg.Proxy.ExternalDir, "Caddyfile"), "local_proxy Caddyfile"),
			checkDockerNetwork(ctx, cfg.Proxy.ExternalNetwork),
			checkManagedSuffix(cfg.LocalTLD),
			checkExternalProxyPort(ctx, cfg.Proxy.ExternalContainerName),
		)
	} else {
		checks = append(checks,
			checkBinary("caddy", "Caddy"),
			checkOptionalBinary("dnsmasq", "dnsmasq", "optional in the current proxy cut; /etc/hosts is used for local resolution"),
			checkTCPPort(ctx, 80),
			checkTCPPort(ctx, 443),
		)
	}

	return Report{Checks: checks}
}

func checkDirectory(path, label string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{
			Name:    label,
			Status:  StatusFail,
			Message: err.Error(),
		}
	}

	if !info.IsDir() {
		return Check{
			Name:    label,
			Status:  StatusFail,
			Message: "expected a directory, found a file",
		}
	}

	return Check{
		Name:    label,
		Status:  StatusOK,
		Message: fmt.Sprintf("found at %s", path),
	}
}

func (r Report) HasFailures() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return true
		}
	}

	return false
}

func (r Report) FailureCount() int {
	count := 0
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			count++
		}
	}

	return count
}

func (r Report) WarningCount() int {
	count := 0
	for _, check := range r.Checks {
		if check.Status == StatusWarn {
			count++
		}
	}

	return count
}

func checkLocalPaths() Check {
	paths, err := config.ResolvePaths()
	if err != nil {
		return Check{
			Name:    "local paths",
			Status:  StatusFail,
			Message: err.Error(),
		}
	}

	if err := paths.Ensure(); err != nil {
		return Check{
			Name:    "local paths",
			Status:  StatusFail,
			Message: err.Error(),
		}
	}

	return Check{
		Name:    "local paths",
		Status:  StatusOK,
		Message: fmt.Sprintf("writable local directories ready at %s", paths.ConfigDir),
	}
}

func checkBinary(binary, label string) Check {
	path, err := exec.LookPath(binary)
	if err != nil {
		return Check{
			Name:    label,
			Status:  StatusFail,
			Message: fmt.Sprintf("%s not found in PATH", binary),
		}
	}

	return Check{
		Name:    label,
		Status:  StatusOK,
		Message: fmt.Sprintf("found at %s", path),
	}
}

func checkOptionalBinary(binary, label, missingMessage string) Check {
	path, err := exec.LookPath(binary)
	if err != nil {
		return Check{
			Name:    label,
			Status:  StatusWarn,
			Message: missingMessage,
		}
	}

	return Check{
		Name:    label,
		Status:  StatusOK,
		Message: fmt.Sprintf("found at %s", path),
	}
}

func checkFile(path, label string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{
			Name:    label,
			Status:  StatusFail,
			Message: err.Error(),
		}
	}

	if info.IsDir() {
		return Check{
			Name:    label,
			Status:  StatusFail,
			Message: "expected a file, found a directory",
		}
	}

	return Check{
		Name:    label,
		Status:  StatusOK,
		Message: fmt.Sprintf("found at %s", path),
	}
}

func checkDockerNetwork(ctx context.Context, network string) Check {
	if network == "" {
		return Check{
			Name:    "shared network",
			Status:  StatusFail,
			Message: "external proxy network is not configured",
		}
	}

	output, err := runCommand(ctx, "docker", "network", "inspect", "--format", "{{.Driver}}\t{{.Scope}}\t{{.Internal}}", network)
	if err == nil {
		info, parseErr := parseDockerNetworkInfo(output)
		if parseErr != nil {
			return Check{
				Name:    "shared network",
				Status:  StatusWarn,
				Message: fmt.Sprintf("docker network %s exists but could not be inspected fully: %s", network, parseErr.Error()),
			}
		}

		if !strings.EqualFold(info.Driver, "bridge") {
			return Check{
				Name:    "shared network",
				Status:  StatusWarn,
				Message: fmt.Sprintf("docker network %s uses driver %s; bridge is recommended for local DevHerd stacks", network, info.Driver),
			}
		}

		if !strings.EqualFold(info.Scope, "local") {
			return Check{
				Name:    "shared network",
				Status:  StatusWarn,
				Message: fmt.Sprintf("docker network %s uses scope %s; local scope is recommended", network, info.Scope),
			}
		}

		if info.Internal {
			return Check{
				Name:    "shared network",
				Status:  StatusWarn,
				Message: fmt.Sprintf("docker network %s is internal; shared local stacks usually need a non-internal bridge network", network),
			}
		}

		return Check{
			Name:    "shared network",
			Status:  StatusOK,
			Message: fmt.Sprintf("docker network %s is ready (%s/%s)", network, info.Driver, info.Scope),
		}
	}

	return Check{
		Name:    "shared network",
		Status:  StatusWarn,
		Message: fmt.Sprintf("docker network %s is missing; DevHerd can create it on first use", network),
	}
}

func checkExternalProxyPort(ctx context.Context, containerName string) Check {
	const proxyPort = 80

	if containerName == "" {
		return Check{
			Name:    "TCP port 80",
			Status:  StatusFail,
			Message: "external proxy container name is not configured",
		}
	}

	listening, err := isTCPPortListening(ctx, proxyPort)
	if err != nil {
		return Check{
			Name:    "TCP port 80",
			Status:  StatusWarn,
			Message: fmt.Sprintf("could not inspect listeners: %s", err.Error()),
		}
	}

	if !listening {
		return Check{
			Name:    "TCP port 80",
			Status:  StatusOK,
			Message: "available",
		}
	}

	output, err := runCommand(ctx, "docker", "ps", "--filter", "name=^/"+containerName+"$", "--format", "{{.Names}}\t{{.Ports}}")
	if err == nil && strings.Contains(output, containerName) {
		return Check{
			Name:    "TCP port 80",
			Status:  StatusOK,
			Message: fmt.Sprintf("already bound by external proxy container %s", containerName),
		}
	}

	return Check{
		Name:    "TCP port 80",
		Status:  StatusWarn,
		Message: "already in use; ensure this is intentional or adjust proxy ports",
	}
}

func checkDockerDaemon(ctx context.Context) Check {
	if _, err := exec.LookPath("docker"); err != nil {
		return Check{
			Name:    "Docker daemon",
			Status:  StatusFail,
			Message: "docker CLI is missing",
		}
	}

	output, err := runCommand(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		return Check{
			Name:    "Docker daemon",
			Status:  StatusFail,
			Message: fmt.Sprintf("docker info failed: %s", err.Error()),
		}
	}

	version := strings.TrimSpace(output)
	if version == "" {
		version = "reachable"
	}

	return Check{
		Name:    "Docker daemon",
		Status:  StatusOK,
		Message: fmt.Sprintf("server %s", version),
	}
}

func checkDockerCompose(ctx context.Context) Check {
	if _, err := exec.LookPath("docker"); err != nil {
		return Check{
			Name:    "Docker Compose",
			Status:  StatusFail,
			Message: "docker CLI is missing",
		}
	}

	output, err := runCommand(ctx, "docker", "compose", "version")
	if err != nil {
		return Check{
			Name:    "Docker Compose",
			Status:  StatusFail,
			Message: fmt.Sprintf("docker compose failed: %s", err.Error()),
		}
	}

	return Check{
		Name:    "Docker Compose",
		Status:  StatusOK,
		Message: firstLine(output),
	}
}

func checkDockerEngineMode(ctx context.Context) Check {
	if _, err := exec.LookPath("docker"); err != nil {
		return Check{
			Name:    "Docker engine mode",
			Status:  StatusFail,
			Message: "docker CLI is missing",
		}
	}

	output, err := runCommand(ctx, "docker", "info", "--format", "{{.OSType}}\t{{.OperatingSystem}}\t{{.Name}}")
	if err != nil {
		return Check{
			Name:    "Docker engine mode",
			Status:  StatusFail,
			Message: fmt.Sprintf("docker info failed: %s", err.Error()),
		}
	}

	info := parseDockerEngineInfo(output)
	if info.OSType == "" {
		return Check{
			Name:    "Docker engine mode",
			Status:  StatusWarn,
			Message: "could not determine docker engine type",
		}
	}

	if !strings.EqualFold(info.OSType, "linux") {
		return Check{
			Name:    "Docker engine mode",
			Status:  StatusFail,
			Message: fmt.Sprintf("DevHerd currently requires Linux containers; current engine type is %s", info.OSType),
		}
	}

	engineLabel := firstNonEmpty(info.OperatingSystem, info.Name, "docker engine")
	return Check{
		Name:    "Docker engine mode",
		Status:  StatusOK,
		Message: fmt.Sprintf("linux containers via %s", engineLabel),
	}
}

func checkTCPPort(ctx context.Context, port int) Check {
	listening, err := isTCPPortListening(ctx, port)
	if err != nil {
		return Check{
			Name:    fmt.Sprintf("TCP port %d", port),
			Status:  StatusWarn,
			Message: fmt.Sprintf("could not inspect listeners: %s", err.Error()),
		}
	}

	if listening {
		return Check{
			Name:    fmt.Sprintf("TCP port %d", port),
			Status:  StatusWarn,
			Message: "already in use; ensure this is intentional or adjust proxy ports",
		}
	}

	return Check{
		Name:    fmt.Sprintf("TCP port %d", port),
		Status:  StatusOK,
		Message: "available",
	}
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, name, args...)
	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out")
	}

	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", err
		}

		return "", fmt.Errorf("%s", firstLine(trimmed))
	}

	return strings.TrimSpace(string(output)), nil
}

func isTCPPortListening(ctx context.Context, port int) (bool, error) {
	switch runtime.GOOS {
	case "linux":
		listening, err := procNetPortListening(port)
		if err == nil {
			return listening, nil
		}

		return lsofPortListening(ctx, port)
	case "windows":
		return windowsNetstatPortListening(ctx, port)
	default:
		return lsofPortListening(ctx, port)
	}
}

func procNetPortListening(port int) (bool, error) {
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		listening, err := scanProcNetTCP(path, port)
		if err != nil {
			return false, err
		}

		if listening {
			return true, nil
		}
	}

	return false, nil
}

func scanProcNetTCP(path string, port int) (bool, error) {
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return false, err
	}

	return procNetContainsListeningPort(string(payload), port), nil
}

func procNetContainsListeningPort(payload string, port int) bool {
	lines := strings.Split(payload, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		if fields[3] != "0A" {
			continue
		}

		localAddress := fields[1]
		parts := strings.Split(localAddress, ":")
		if len(parts) != 2 {
			continue
		}

		value, err := strconv.ParseInt(parts[1], 16, 32)
		if err != nil {
			continue
		}

		if int(value) == port {
			return true
		}
	}

	return false
}

func lsofPortListening(ctx context.Context, port int) (bool, error) {
	if _, err := exec.LookPath("lsof"); err != nil {
		return false, fmt.Errorf("lsof not found in PATH")
	}

	output, err := runOptionalMatchCommand(ctx, "lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(output) != "", nil
}

func windowsNetstatPortListening(ctx context.Context, port int) (bool, error) {
	if _, err := exec.LookPath("netstat"); err != nil {
		return false, fmt.Errorf("netstat not found in PATH")
	}

	output, err := runCommand(ctx, "netstat", "-ano", "-p", "tcp")
	if err != nil {
		return false, err
	}

	return windowsNetstatContainsListeningPort(output, port), nil
}

func windowsNetstatContainsListeningPort(output string, port int) bool {
	suffix := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}

		if !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}

		if strings.HasSuffix(fields[1], suffix) {
			return true
		}
	}

	return false
}

func runOptionalMatchCommand(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, name, args...)
	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out")
	}

	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(string(output)) == "" {
		return "", nil
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return "", err
	}

	return "", fmt.Errorf("%s", firstLine(trimmed))
}

func parseDockerEngineInfo(output string) dockerEngineInfo {
	fields := strings.Split(strings.TrimSpace(output), "\t")
	info := dockerEngineInfo{}
	if len(fields) > 0 {
		info.OSType = strings.TrimSpace(fields[0])
	}
	if len(fields) > 1 {
		info.OperatingSystem = strings.TrimSpace(fields[1])
	}
	if len(fields) > 2 {
		info.Name = strings.TrimSpace(fields[2])
	}

	return info
}

func parseDockerNetworkInfo(output string) (dockerNetworkInfo, error) {
	fields := strings.Split(strings.TrimSpace(output), "\t")
	if len(fields) < 3 {
		return dockerNetworkInfo{}, fmt.Errorf("unexpected docker network inspect output")
	}

	internal, err := strconv.ParseBool(strings.TrimSpace(fields[2]))
	if err != nil {
		return dockerNetworkInfo{}, fmt.Errorf("parse network internal flag: %w", err)
	}

	return dockerNetworkInfo{
		Driver:   strings.TrimSpace(fields[0]),
		Scope:    strings.TrimSpace(fields[1]),
		Internal: internal,
	}, nil
}

func checkManagedSuffix(localTLD string) Check {
	return checkManagedSuffixForOS(runtime.GOOS, localTLD)
}

func checkManagedSuffixForOS(goos, localTLD string) Check {
	if goos != "darwin" && goos != "windows" {
		return Check{
			Name:    "managed suffix",
			Status:  StatusOK,
			Message: fmt.Sprintf(".%s", strings.TrimPrefix(localTLD, ".")),
		}
	}

	suffix := strings.TrimPrefix(localTLD, ".")
	if strings.EqualFold(suffix, "localhost") {
		return Check{
			Name:    "managed suffix",
			Status:  StatusOK,
			Message: ".localhost works without extra DNS setup on this OS",
		}
	}

	return Check{
		Name:    "managed suffix",
		Status:  StatusWarn,
		Message: fmt.Sprintf(".%s may require manual DNS or hosts setup on %s; prefer .localhost for the external proxy", suffix, goos),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func firstLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "ok"
	}

	return lines[0]
}
