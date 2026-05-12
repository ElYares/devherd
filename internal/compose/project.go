package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var supportedComposeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

const manifestFileName = ".devherd.yml"

const (
	ProjectSourceAutodetect = "autodetect"
	ProjectSourceManifest   = "manifest"
)

type Project struct {
	Root         string
	ComposeFiles []string
	EnvFile      string
	Source       string
	Proxy        ProjectProxy
}

type manifest struct {
	Version int             `yaml:"version"`
	Compose manifestCompose `yaml:"compose"`
	Proxy   manifestProxy   `yaml:"proxy"`
}

type manifestCompose struct {
	Files   []string `yaml:"files"`
	EnvFile string   `yaml:"env_file"`
}

type manifestProxy struct {
	Domain  string `yaml:"domain"`
	Service string `yaml:"service"`
	Port    int    `yaml:"port"`
}

type ProjectProxy struct {
	Domain  string
	Service string
	Port    int
}

func ResolveProject(input string) (Project, error) {
	target := input
	if target == "" {
		var err error
		target, err = os.Getwd()
		if err != nil {
			return Project{}, fmt.Errorf("resolve current directory: %w", err)
		}
	}

	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return Project{}, fmt.Errorf("resolve project path: %w", err)
	}

	info, err := os.Stat(absoluteTarget)
	if err != nil {
		return Project{}, fmt.Errorf("stat project path: %w", err)
	}

	if !info.IsDir() {
		return Project{}, fmt.Errorf("project path must be a directory")
	}

	if project, ok, err := resolveManifestProject(absoluteTarget); err != nil {
		return Project{}, err
	} else if ok {
		return project, nil
	}

	for _, candidate := range supportedComposeFiles {
		composeFile := filepath.Join(absoluteTarget, candidate)
		if _, err := os.Stat(composeFile); err == nil {
			return Project{
				Root:         absoluteTarget,
				ComposeFiles: []string{composeFile},
				Source:       ProjectSourceAutodetect,
			}, nil
		}
	}

	return Project{}, fmt.Errorf("no supported compose file found in %s", absoluteTarget)
}

func Up(ctx context.Context, projectPath string) (string, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return "", err
	}

	return UpProject(ctx, project)
}

func Down(ctx context.Context, projectPath string) (string, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return "", err
	}

	return DownProject(ctx, project)
}

func Stop(ctx context.Context, projectPath string) (string, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return "", err
	}

	return StopProject(ctx, project)
}

func UpProject(ctx context.Context, project Project) (string, error) {
	args := composeArgs(project)
	args = append(args, "up", "--build", "-d")
	return run(ctx, project.Root, "docker", args...)
}

func DownProject(ctx context.Context, project Project) (string, error) {
	args := composeArgs(project)
	args = append(args, "down")
	return run(ctx, project.Root, "docker", args...)
}

func StopProject(ctx context.Context, project Project) (string, error) {
	args := composeArgs(project)
	args = append(args, "stop")
	return run(ctx, project.Root, "docker", args...)
}

func resolveManifestProject(root string) (Project, bool, error) {
	manifestPath := filepath.Join(root, manifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Project{}, false, nil
		}

		return Project{}, false, fmt.Errorf("read %s: %w", manifestFileName, err)
	}

	var cfg manifest
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Project{}, false, fmt.Errorf("parse %s: %w", manifestFileName, err)
	}

	if len(cfg.Compose.Files) == 0 {
		return Project{}, false, fmt.Errorf("%s requires compose.files", manifestFileName)
	}

	composeFiles := make([]string, 0, len(cfg.Compose.Files))
	for _, file := range cfg.Compose.Files {
		resolved, err := resolveRelativeFile(root, file)
		if err != nil {
			return Project{}, false, fmt.Errorf("%s compose file %q: %w", manifestFileName, file, err)
		}
		composeFiles = append(composeFiles, resolved)
	}

	project := Project{
		Root:         root,
		ComposeFiles: composeFiles,
		Source:       ProjectSourceManifest,
		Proxy: ProjectProxy{
			Domain:  cfg.Proxy.Domain,
			Service: cfg.Proxy.Service,
			Port:    cfg.Proxy.Port,
		},
	}

	if cfg.Compose.EnvFile != "" {
		resolved, err := resolveRelativeFile(root, cfg.Compose.EnvFile)
		if err != nil {
			return Project{}, false, fmt.Errorf("%s env_file %q: %w", manifestFileName, cfg.Compose.EnvFile, err)
		}

		project.EnvFile = resolved
	}

	return project, true, nil
}

func resolveRelativeFile(root string, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}

	if filepath.IsAbs(value) {
		return "", fmt.Errorf("must be a relative path")
	}

	resolved := filepath.Join(root, filepath.Clean(value))
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist")
		}

		return "", err
	}

	if info.IsDir() {
		return "", fmt.Errorf("must reference a file")
	}

	return resolved, nil
}

func composeArgs(project Project) []string {
	args := []string{"compose"}

	if project.EnvFile != "" {
		args = append(args, "--env-file", project.EnvFile)
	}

	for _, composeFile := range project.ComposeFiles {
		args = append(args, "-f", composeFile)
	}

	return args
}

func Plan(projectPath string) (Project, []string, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return Project{}, nil, err
	}

	return PlanProject(project), append([]string{"docker"}, composeArgs(project)...), nil
}

func PlanProject(project Project) Project {
	return project
}

func Command(project Project) []string {
	args := append([]string{"docker"}, composeArgs(project)...)
	return args
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
