package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func Run(ctx context.Context) Report {
	return RunWithConfig(ctx, config.Default())
}

func RunWithConfig(ctx context.Context, cfg config.Config) Report {
	checks := []Check{
		checkLocalPaths(),
		checkBinary("docker", "Docker CLI"),
		checkDockerDaemon(ctx),
		checkDockerCompose(ctx),
	}

	if cfg.Proxy.Driver == "caddy-docker-external" {
		checks = append(checks,
			checkFile(filepath.Join("/home/elyarestark/infra/local_proxy", "docker-compose.yml"), "local_proxy compose"),
			checkFile(filepath.Join("/home/elyarestark/infra/local_proxy", "Caddyfile"), "local_proxy Caddyfile"),
			checkTCPPort(80),
		)
	} else {
		checks = append(checks,
			checkBinary("caddy", "Caddy"),
			checkOptionalBinary("dnsmasq", "dnsmasq", "optional in the current proxy cut; /etc/hosts is used for local resolution"),
			checkTCPPort(80),
			checkTCPPort(443),
		)
	}

	return Report{Checks: checks}
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
		Message: fmt.Sprintf("writable XDG directories ready at %s", paths.ConfigDir),
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

func checkTCPPort(port int) Check {
	listening, err := isTCPPortListening(port)
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

func isTCPPortListening(port int) (bool, error) {
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

	lines := strings.Split(string(payload), "\n")
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
			return true, nil
		}
	}

	return false, nil
}

func firstLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "ok"
	}

	return lines[0]
}
