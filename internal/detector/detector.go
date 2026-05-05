package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type Project struct {
	Name      string
	Path      string
	Stack     string
	Framework string
	Runtime   string
}

type featureSet struct {
	laravel bool
	node    bool
	vue     bool
	python  bool
	flask   bool
	goLang  bool
	docker  bool
}

func Discover(root string) ([]Project, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	candidates := []string{root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read root directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || shouldSkipDirectory(entry.Name()) {
			continue
		}

		candidates = append(candidates, filepath.Join(root, entry.Name()))
	}

	var projects []Project
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		project, ok, err := DetectProject(candidate)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		if _, exists := seen[project.Path]; exists {
			continue
		}

		seen[project.Path] = struct{}{}
		projects = append(projects, project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

func DetectProject(path string) (Project, bool, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return Project{}, false, fmt.Errorf("resolve project path: %w", err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return Project{}, false, fmt.Errorf("stat project path: %w", err)
	}

	if !info.IsDir() {
		return Project{}, false, nil
	}

	features := featureSet{}
	if err := inspectDirectory(absolutePath, true, &features); err != nil {
		return Project{}, false, err
	}

	children, err := os.ReadDir(absolutePath)
	if err != nil {
		return Project{}, false, fmt.Errorf("read project directory: %w", err)
	}

	for _, child := range children {
		if !child.IsDir() || strings.HasPrefix(child.Name(), ".") || shouldSkipDirectory(child.Name()) {
			continue
		}

		if err := inspectDirectory(filepath.Join(absolutePath, child.Name()), false, &features); err != nil {
			return Project{}, false, err
		}
	}

	if !hasAnyFeature(features) {
		return Project{}, false, nil
	}

	return Project{
		Name:      filepath.Base(absolutePath),
		Path:      absolutePath,
		Stack:     describeStack(features),
		Framework: describeFramework(features),
		Runtime:   describeRuntime(features),
	}, true, nil
}

func inspectDirectory(path string, isRoot bool, features *featureSet) error {
	if fileExists(filepath.Join(path, "artisan")) && fileExists(filepath.Join(path, "composer.json")) {
		features.laravel = true
	}

	if fileExists(filepath.Join(path, "package.json")) {
		features.node = true

		hasVue, err := packageJSONHasDependency(filepath.Join(path, "package.json"), "vue")
		if err != nil {
			return fmt.Errorf("inspect package.json: %w", err)
		}

		if hasVue {
			features.vue = true
		}
	}

	if fileExists(filepath.Join(path, "requirements.txt")) || fileExists(filepath.Join(path, "pyproject.toml")) {
		features.python = true
	}

	if fileExists(filepath.Join(path, "requirements.txt")) {
		hasFlask, err := textFileContains(filepath.Join(path, "requirements.txt"), "flask")
		if err != nil {
			return fmt.Errorf("inspect requirements.txt: %w", err)
		}

		if hasFlask {
			features.flask = true
		}
	}

	if fileExists(filepath.Join(path, "pyproject.toml")) {
		hasFlask, err := textFileContains(filepath.Join(path, "pyproject.toml"), "flask")
		if err != nil {
			return fmt.Errorf("inspect pyproject.toml: %w", err)
		}

		if hasFlask {
			features.flask = true
		}
	}

	if fileExists(filepath.Join(path, "app.py")) {
		hasFlask, err := textFileContains(filepath.Join(path, "app.py"), "from flask import")
		if err != nil {
			return fmt.Errorf("inspect app.py: %w", err)
		}

		if hasFlask {
			features.flask = true
			features.python = true
		}
	}

	if fileExists(filepath.Join(path, "go.mod")) {
		features.goLang = true
	}

	if isRoot && (fileExists(filepath.Join(path, "docker-compose.yml")) ||
		fileExists(filepath.Join(path, "docker-compose.yaml")) ||
		fileExists(filepath.Join(path, "compose.yml")) ||
		fileExists(filepath.Join(path, "compose.yaml")) ||
		fileExists(filepath.Join(path, "Dockerfile"))) {
		features.docker = true
	}

	return nil
}

func hasAnyFeature(features featureSet) bool {
	return features.laravel || features.node || features.vue || features.python || features.flask || features.goLang || features.docker
}

func shouldSkipDirectory(name string) bool {
	switch name {
	case "node_modules":
		return true
	default:
		return false
	}
}

func describeStack(features featureSet) string {
	var parts []string

	if features.laravel {
		parts = append(parts, "php")
	}

	if features.node {
		parts = append(parts, "node")
	}

	if features.python {
		parts = append(parts, "python")
	}

	if features.goLang {
		parts = append(parts, "go")
	}

	if features.docker {
		parts = append(parts, "docker")
	}

	return strings.Join(parts, "+")
}

func describeFramework(features featureSet) string {
	switch {
	case features.vue && features.flask:
		return "vue+flask"
	case features.laravel:
		return "laravel"
	case features.vue:
		return "vue"
	case features.flask:
		return "flask"
	case features.python:
		return "python"
	case features.goLang:
		return "go"
	case features.node:
		return "node"
	case features.docker:
		return "docker"
	default:
		return "unknown"
	}
}

func describeRuntime(features featureSet) string {
	var runtimes []string

	if features.laravel {
		runtimes = append(runtimes, "php")
	}

	if features.node {
		runtimes = append(runtimes, "node")
	}

	if features.python {
		runtimes = append(runtimes, "python")
	}

	if features.goLang {
		runtimes = append(runtimes, "go")
	}

	return strings.Join(runtimes, "+")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func textFileContains(path, needle string) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	return strings.Contains(strings.ToLower(string(payload)), strings.ToLower(needle)), nil
}

func packageJSONHasDependency(path, dependency string) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var raw struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(payload, &raw); err != nil {
		return false, err
	}

	return slices.Contains(mapsKeys(raw.Dependencies), dependency) || slices.Contains(mapsKeys(raw.DevDependencies), dependency), nil
}

func mapsKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	return keys
}
