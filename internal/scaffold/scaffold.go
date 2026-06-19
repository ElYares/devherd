// Package scaffold genera un docker-compose (+ manifiesto .devherd.yml) para
// proyectos que no traen contenedores, a partir del stack detectado. Prototipo:
// soporta el layout vue+flask (un hijo Flask como backend y un hijo Vue como
// frontend), generando un compose dev-friendly con imágenes base y bind-mounts
// (sin requerir Dockerfiles).
package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ManagedComposeFile es el compose generado por DevHerd. Se usa un nombre
	// propio para no pisar un docker-compose.yml del usuario.
	ManagedComposeFile = "docker-compose.devherd.yml"
	ManifestFile       = ".devherd.yml"
)

// Service es un servicio del compose generado.
type Service struct {
	Name       string
	Image      string
	WorkingDir string
	Mount      string // host:container
	Command    string
	Env        map[string]string
	Ports      []string // host:container
}

// Plan es el resultado de analizar un proyecto: qué servicios generar.
type Plan struct {
	Root      string
	Framework string
	Services  []Service
}

// Detect analiza el proyecto y produce un Plan. Devuelve error si el layout no
// está soportado por el prototipo.
func Detect(root string) (Plan, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve path: %w", err)
	}

	backendDir := findChild(abs, isFlaskDir)
	frontendDir := findChild(abs, isVueDir)

	if backendDir != "" && frontendDir != "" {
		return Plan{
			Root:      abs,
			Framework: "vue+flask",
			Services: []Service{
				flaskService("backend", filepath.Base(backendDir)),
				vueService("frontend", filepath.Base(frontendDir)),
			},
		}, nil
	}

	return Plan{}, fmt.Errorf("scaffold: layout no soportado en %s (el prototipo soporta vue+flask: un hijo Flask y un hijo Vue)", abs)
}

func flaskService(name, dir string) Service {
	return Service{
		Name:       name,
		Image:      "python:3.12-slim",
		WorkingDir: "/app",
		Mount:      "./" + dir + ":/app",
		Command:    `sh -c "pip install -r requirements.txt && flask run --host 0.0.0.0 --port 8000"`,
		Env: map[string]string{
			"FLASK_APP":      "app.py",
			"FLASK_RUN_PORT": "8000",
		},
		Ports: []string{"8000:8000"},
	}
}

func vueService(name, dir string) Service {
	return Service{
		Name:       name,
		Image:      "node:20-alpine",
		WorkingDir: "/app",
		Mount:      "./" + dir + ":/app",
		Command:    `sh -c "npm install && npm run dev -- --host 0.0.0.0 --port 5173"`,
		Ports:      []string{"5173:5173"},
	}
}

// RenderCompose genera el YAML del docker-compose a partir del Plan.
func RenderCompose(plan Plan) string {
	var b strings.Builder
	b.WriteString("# Generado por DevHerd (devherd scaffold). Edítalo a tu gusto.\n")
	b.WriteString("services:\n")

	for _, svc := range plan.Services {
		fmt.Fprintf(&b, "  %s:\n", svc.Name)
		fmt.Fprintf(&b, "    image: %s\n", svc.Image)
		if svc.WorkingDir != "" {
			fmt.Fprintf(&b, "    working_dir: %s\n", svc.WorkingDir)
		}
		if svc.Mount != "" {
			b.WriteString("    volumes:\n")
			fmt.Fprintf(&b, "      - %s\n", svc.Mount)
		}
		if svc.Command != "" {
			fmt.Fprintf(&b, "    command: %s\n", svc.Command)
		}
		if len(svc.Env) > 0 {
			b.WriteString("    environment:\n")
			for _, k := range sortedKeys(svc.Env) {
				fmt.Fprintf(&b, "      %s: %q\n", k, svc.Env[k])
			}
		}
		if len(svc.Ports) > 0 {
			b.WriteString("    ports:\n")
			for _, p := range svc.Ports {
				fmt.Fprintf(&b, "      - %q\n", p)
			}
		}
	}

	return b.String()
}

// RenderManifest genera el .devherd.yml que apunta al compose gestionado. No
// fija proxy.service/port: para vue+flask el ruteo lo resuelve el framework
// detectado en proxy.BuildExternalProject.
func RenderManifest(plan Plan) string {
	var b strings.Builder
	b.WriteString("# Generado por DevHerd (devherd scaffold).\n")
	b.WriteString("version: 1\n")
	b.WriteString("compose:\n")
	b.WriteString("  files:\n")
	fmt.Fprintf(&b, "    - %s\n", ManagedComposeFile)
	return b.String()
}

// Write escribe el compose y el manifiesto en el root del proyecto. No
// sobrescribe archivos existentes salvo que force sea true. Devuelve las rutas
// escritas (o reusadas) y el estado por archivo.
func Write(plan Plan, force bool) (map[string]string, error) {
	results := make(map[string]string, 2)

	files := []struct {
		name    string
		content string
	}{
		{ManagedComposeFile, RenderCompose(plan)},
		{ManifestFile, RenderManifest(plan)},
	}

	for _, f := range files {
		path := filepath.Join(plan.Root, f.name)
		state, err := writeManaged(path, f.content, force)
		if err != nil {
			return results, err
		}
		results[f.name] = state
	}

	return results, nil
}

func writeManaged(path, content string, force bool) (string, error) {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return "skipped (ya existe)", nil
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
		return "updated", nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return "created", nil
}

// --- helpers de detección ---

func findChild(root string, match func(dir string) bool) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if match(dir) {
			return dir
		}
	}

	return ""
}

func isFlaskDir(dir string) bool {
	if fileContains(filepath.Join(dir, "requirements.txt"), "flask") {
		return true
	}
	if fileExists(filepath.Join(dir, "app.py")) && fileContains(filepath.Join(dir, "app.py"), "flask") {
		return true
	}
	return false
}

func isVueDir(dir string) bool {
	return packageJSONHasDependency(filepath.Join(dir, "package.json"), "vue")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileContains(path, needle string) bool {
	payload, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(payload)), strings.ToLower(needle))
}

func packageJSONHasDependency(path, dep string) bool {
	payload, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var raw struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return false
	}

	if _, ok := raw.Dependencies[dep]; ok {
		return true
	}
	_, ok := raw.DevDependencies[dep]
	return ok
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
