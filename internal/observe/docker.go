package observe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type DockerRuntime interface {
	ObservedContainers(ctx context.Context, project string) ([]ObservedContainer, error)
	LogsAround(ctx context.Context, container string, at time.Time, window time.Duration, limit int) ([]ContainerLog, error)
}

type DockerCLI struct{}

type ObservedContainer struct {
	ContainerID  string            `json:"container_id"`
	Name         string            `json:"name"`
	Project      string            `json:"project"`
	Service      string            `json:"service"`
	Image        string            `json:"image"`
	Status       string            `json:"status"`
	RestartCount int               `json:"restart_count"`
	Labels       map[string]string `json:"labels"`
}

type ContainerEvent struct {
	ID           int64  `json:"id"`
	ContainerID  string `json:"container_id"`
	Name         string `json:"name"`
	Project      string `json:"project"`
	Service      string `json:"service"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	RestartCount int    `json:"restart_count"`
	Message      string `json:"message"`
	CreatedAt    string `json:"created_at"`
}

type ContainerLog struct {
	EventID   string `json:"event_id"`
	Project   string `json:"project"`
	Service   string `json:"service"`
	Container string `json:"container"`
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Message   string `json:"message"`
}

type inspectContainer struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	RestartCount int    `json:"RestartCount"`
	Config       struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
}

func (DockerCLI) ObservedContainers(ctx context.Context, project string) ([]ObservedContainer, error) {
	args := []string{"ps", "-aq", "--filter", "label=devherd.observe=true"}
	if strings.TrimSpace(project) != "" {
		args = append(args, "--filter", "label=devherd.project="+project)
	}

	output, err := runDocker(ctx, args...)
	if err != nil {
		return nil, err
	}

	ids := strings.Fields(output)
	if len(ids) == 0 {
		return nil, nil
	}

	inspectArgs := append([]string{"inspect"}, ids...)
	inspectOutput, err := runDocker(ctx, inspectArgs...)
	if err != nil {
		return nil, err
	}

	return parseObservedContainers(inspectOutput)
}

func (DockerCLI) LogsAround(ctx context.Context, container string, at time.Time, window time.Duration, limit int) ([]ContainerLog, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return nil, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if window <= 0 {
		window = 30 * time.Second
	}
	if limit <= 0 {
		limit = 200
	}

	since := at.Add(-window).UTC().Format(time.RFC3339Nano)
	until := at.Add(window).UTC().Format(time.RFC3339Nano)
	output, err := runDocker(ctx, "logs", "--timestamps", "--since", since, "--until", until, "--tail", strconv.Itoa(limit), container)
	if err != nil {
		return nil, err
	}

	return parseDockerLogs(output), nil
}

func runDocker(ctx context.Context, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("docker timed out")
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

func firstLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "docker command failed"
	}

	return lines[0]
}

func parseObservedContainers(payload string) ([]ObservedContainer, error) {
	if strings.TrimSpace(payload) == "" {
		return nil, nil
	}

	var raw []inspectContainer
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, fmt.Errorf("parse docker inspect: %w", err)
	}

	containers := make([]ObservedContainer, 0, len(raw))
	for _, item := range raw {
		labels := item.Config.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		containers = append(containers, ObservedContainer{
			ContainerID:  item.ID,
			Name:         strings.TrimPrefix(item.Name, "/"),
			Project:      labels["devherd.project"],
			Service:      labels["devherd.service"],
			Image:        item.Config.Image,
			Status:       item.State.Status,
			RestartCount: item.RestartCount,
			Labels:       labels,
		})
	}

	return containers, nil
}

func parseDockerLogs(output string) []ContainerLog {
	var logs []ContainerLog
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		timestamp, message, ok := strings.Cut(line, " ")
		if !ok || !looksLikeDockerTimestamp(timestamp) {
			message = line
			timestamp = ""
		}

		logs = append(logs, ContainerLog{
			Timestamp: strings.TrimSpace(timestamp),
			Stream:    "combined",
			Message:   strings.TrimSpace(message),
		})
	}

	return logs
}

func looksLikeDockerTimestamp(value string) bool {
	if value == "" {
		return false
	}

	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}

	return false
}
