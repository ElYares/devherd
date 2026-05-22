package compose

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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
	Root              string
	ComposeFiles      []string
	EnvFile           string
	Source            string
	ProjectName       string
	LegacyProjectName string
	Proxy             ProjectProxy
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
		return withProjectNames(project), nil
	}

	for _, candidate := range supportedComposeFiles {
		composeFile := filepath.Join(absoluteTarget, candidate)
		if _, err := os.Stat(composeFile); err == nil {
			return Project{
				Root:         absoluteTarget,
				ComposeFiles: []string{composeFile},
				Source:       ProjectSourceAutodetect,
			}.withProjectNames(), nil
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
	output, err := run(ctx, project.Root, "docker", args...)
	if err != nil {
		return output, err
	}

	if project.LegacyProjectName == "" || project.LegacyProjectName == project.ProjectName {
		return output, nil
	}

	legacyProject := project
	legacyProject.ProjectName = project.LegacyProjectName
	legacyArgs := composeArgs(legacyProject)
	legacyArgs = append(legacyArgs, "down")
	legacyOutput, legacyErr := run(ctx, project.Root, "docker", legacyArgs...)
	if legacyOutput != "" {
		output = joinOutput(output, legacyOutput)
	}
	if legacyErr != nil {
		return output, legacyErr
	}

	return output, nil
}

func StopProject(ctx context.Context, project Project) (string, error) {
	args := composeArgs(project)
	args = append(args, "stop")
	output, err := run(ctx, project.Root, "docker", args...)
	if err != nil {
		return output, err
	}

	if project.LegacyProjectName == "" || project.LegacyProjectName == project.ProjectName {
		return output, nil
	}

	legacyProject := project
	legacyProject.ProjectName = project.LegacyProjectName
	legacyArgs := composeArgs(legacyProject)
	legacyArgs = append(legacyArgs, "stop")
	legacyOutput, legacyErr := run(ctx, project.Root, "docker", legacyArgs...)
	if legacyOutput != "" {
		output = joinOutput(output, legacyOutput)
	}
	if legacyErr != nil {
		return output, legacyErr
	}

	return output, nil
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

	if project.ProjectName != "" {
		args = append(args, "--project-name", project.ProjectName)
	}

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

func (project Project) withProjectNames() Project {
	return withProjectNames(project)
}

func withProjectNames(project Project) Project {
	if project.Root == "" {
		return project
	}

	if project.ProjectName == "" {
		project.ProjectName = ProjectNameForPath(project.Root)
	}
	if project.LegacyProjectName == "" {
		project.LegacyProjectName = LegacyProjectNameForPath(project.Root)
	}

	return project
}

func ProjectNameForPath(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = filepath.Clean(path)
	}

	sum := sha1.Sum([]byte(absolutePath))
	hash := hex.EncodeToString(sum[:])[:8]
	slug := composeProjectLabel(filepath.Base(absolutePath))
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	if slug == "" {
		slug = "project"
	}

	return "devherd-" + slug + "-" + hash
}

func LegacyProjectNameForPath(path string) string {
	return composeProjectLabel(filepath.Base(filepath.Clean(path)))
}

func composeProjectLabel(value string) string {
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

func joinOutput(left, right string) string {
	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return left + "\n" + right
	}
}
