package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var supportedComposeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

func ResolveProject(input string) (string, string, error) {
	target := input
	if target == "" {
		var err error
		target, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("resolve current directory: %w", err)
		}
	}

	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve project path: %w", err)
	}

	info, err := os.Stat(absoluteTarget)
	if err != nil {
		return "", "", fmt.Errorf("stat project path: %w", err)
	}

	if !info.IsDir() {
		return "", "", fmt.Errorf("project path must be a directory")
	}

	for _, candidate := range supportedComposeFiles {
		composeFile := filepath.Join(absoluteTarget, candidate)
		if _, err := os.Stat(composeFile); err == nil {
			return absoluteTarget, composeFile, nil
		}
	}

	return "", "", fmt.Errorf("no supported compose file found in %s", absoluteTarget)
}

func Up(ctx context.Context, projectPath string) (string, error) {
	root, composeFile, err := ResolveProject(projectPath)
	if err != nil {
		return "", err
	}

	return run(ctx, root, "docker", "compose", "-f", composeFile, "up", "--build", "-d")
}

func Down(ctx context.Context, projectPath string) (string, error) {
	root, composeFile, err := ResolveProject(projectPath)
	if err != nil {
		return "", err
	}

	return run(ctx, root, "docker", "compose", "-f", composeFile, "down")
}

func run(ctx context.Context, workdir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", err
		}

		return "", fmt.Errorf("%s", trimmed)
	}

	return strings.TrimSpace(string(output)), nil
}
