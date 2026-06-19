// Package scaffold genera un docker-compose (+ manifiesto .devherd.yml) para
// proyectos que no traen contenedores, a partir del stack detectado.
//
// Diseño anti-colisión: las bases de datos y Redis quedan internas a la red del
// proyecto (la app se conecta por nombre de servicio, sin publicar puertos), y
// los servicios de aplicación publican en puertos de host LIBRES autodetectados.
package scaffold

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	ManagedComposeFile = "docker-compose.devherd.yml"
	ManifestFile       = ".devherd.yml"
)

// Service es un servicio del compose generado.
type Service struct {
	Name          string
	Image         string
	WorkingDir    string
	Volumes       []string // "./backend:/app" (bind) o "db_data:/var/lib/mysql" (named)
	Command       string
	Env           map[string]string
	DependsOn     []string
	ContainerPort int  // puerto dentro del contenedor
	HostPort      int  // puerto de host asignado (0 = sin asignar / no publicado)
	Publish       bool // si true, se publica en un puerto de host libre
	Backing       bool // servicio de respaldo (db/redis), no de aplicación
}

// Plan es el resultado de analizar un proyecto.
type Plan struct {
	Root      string
	Framework string
	Services  []Service
}

// SupportedDatabases lista las opciones de base de datos del menú.
var SupportedDatabases = []string{"mysql", "mariadb", "postgres", "mongodb", "none"}

// Detect analiza el proyecto y produce un Plan con los servicios de aplicación.
func Detect(root string) (Plan, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve path: %w", err)
	}

	// 1) Combo multi-servicio: un hijo Flask (backend) + un hijo Vue (frontend).
	if backend := findChild(abs, isFlaskDir); backend != "" {
		if frontend := findChild(abs, isVueDir); frontend != "" {
			return Plan{
				Root:      abs,
				Framework: "vue+flask",
				Services: []Service{
					flaskService("backend", filepath.Base(backend)),
					vueService("frontend", filepath.Base(frontend)),
				},
			}, nil
		}
	}

	// 2) Stacks de un solo servicio en la raíz.
	switch {
	case isLaravelDir(abs):
		return single(abs, "laravel", laravelService(abs)), nil
	case isVueDir(abs):
		return single(abs, "vue", vueService("app", ".")), nil
	case isFlaskDir(abs):
		return single(abs, "flask", flaskService("app", ".")), nil
	case isNodeDir(abs):
		return single(abs, "node", nodeService(abs)), nil
	case isGoDir(abs):
		return single(abs, "go", goService(abs)), nil
	}

	return Plan{}, fmt.Errorf("scaffold: no reconozco el stack en %s (soportados: vue+flask, laravel, vue, flask, node, go)", abs)
}

func single(root, framework string, svc Service) Plan {
	return Plan{Root: root, Framework: framework, Services: []Service{svc}}
}

// --- servicios de aplicación por stack ---

func flaskService(name, dir string) Service {
	return Service{
		Name:          name,
		Image:         "python:3.12-slim",
		WorkingDir:    "/app",
		Volumes:       []string{mount(dir)},
		Command:       `sh -c "pip install -r requirements.txt && flask run --host 0.0.0.0 --port 8000"`,
		Env:           map[string]string{"FLASK_APP": "app.py", "FLASK_RUN_PORT": "8000"},
		ContainerPort: 8000,
		Publish:       true,
	}
}

func vueService(name, dir string) Service {
	return Service{
		Name:          name,
		Image:         "node:20-alpine",
		WorkingDir:    "/app",
		Volumes:       []string{mount(dir)},
		Command:       `sh -c "npm install && npm run dev -- --host 0.0.0.0 --port 5173"`,
		ContainerPort: 5173,
		Publish:       true,
	}
}

func nodeService(dir string) Service {
	script := "start"
	if packageJSONHasScript(filepath.Join(dir, "package.json"), "dev") {
		script = "dev"
	}
	return Service{
		Name:          "app",
		Image:         "node:20-alpine",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       fmt.Sprintf(`sh -c "npm install && npm run %s"`, script),
		Env:           map[string]string{"PORT": "3000"},
		ContainerPort: 3000,
		Publish:       true,
	}
}

func goService(_ string) Service {
	return Service{
		Name:          "app",
		Image:         "golang:1.25",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       `sh -c "go run ./..."`,
		ContainerPort: 8080,
		Publish:       true,
	}
}

func laravelService(_ string) Service {
	return Service{
		Name:          "app",
		Image:         "php:8.3-cli",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       `sh -c "php artisan serve --host 0.0.0.0 --port 8000"`,
		ContainerPort: 8000,
		Publish:       true,
	}
}

func mount(dir string) string {
	if dir == "." || dir == "" {
		return "./:/app"
	}
	return "./" + dir + ":/app"
}

// --- servicios de respaldo ---

// AddDatabase añade un servicio de base de datos (interno) y cablea las
// variables de entorno de conexión en los servicios de aplicación.
func (p *Plan) AddDatabase(kind string) error {
	svc, appEnv, ok := databaseService(kind)
	if !ok {
		if kind == "none" || kind == "" {
			return nil
		}
		return fmt.Errorf("base de datos no soportada %q (opciones: %s)", kind, strings.Join(SupportedDatabases, ", "))
	}

	p.Services = append(p.Services, svc)
	p.wireBacking(svc.Name, appEnv)
	return nil
}

// AddRedis añade Redis (interno) y cablea REDIS_HOST/PORT en la app.
func (p *Plan) AddRedis() {
	svc := Service{Name: "redis", Image: "redis:7-alpine", ContainerPort: 6379, Backing: true}
	p.Services = append(p.Services, svc)
	p.wireBacking("redis", map[string]string{"REDIS_HOST": "redis", "REDIS_PORT": "6379"})
}

func (p *Plan) wireBacking(name string, appEnv map[string]string) {
	for i := range p.Services {
		svc := &p.Services[i]
		if svc.Backing {
			continue
		}
		svc.DependsOn = append(svc.DependsOn, name)
		if svc.Env == nil {
			svc.Env = map[string]string{}
		}
		for k, v := range appEnv {
			svc.Env[k] = v
		}
	}
}

func databaseService(kind string) (Service, map[string]string, bool) {
	switch kind {
	case "mysql", "mariadb":
		image := "mysql:8"
		if kind == "mariadb" {
			image = "mariadb:11"
		}
		svc := Service{
			Name:    "db",
			Image:   image,
			Volumes: []string{"db_data:/var/lib/mysql"},
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "devherd",
				"MYSQL_DATABASE":      "app",
				"MYSQL_USER":          "app",
				"MYSQL_PASSWORD":      "devherd",
			},
			ContainerPort: 3306,
			Backing:       true,
		}
		appEnv := map[string]string{
			"DB_CONNECTION": "mysql", "DB_HOST": "db", "DB_PORT": "3306",
			"DB_DATABASE": "app", "DB_USERNAME": "app", "DB_PASSWORD": "devherd",
			"DATABASE_URL": "mysql://app:devherd@db:3306/app",
		}
		return svc, appEnv, true

	case "postgres":
		svc := Service{
			Name:    "db",
			Image:   "postgres:16",
			Volumes: []string{"db_data:/var/lib/postgresql/data"},
			Env: map[string]string{
				"POSTGRES_DB": "app", "POSTGRES_USER": "app", "POSTGRES_PASSWORD": "devherd",
			},
			ContainerPort: 5432,
			Backing:       true,
		}
		appEnv := map[string]string{
			"DB_CONNECTION": "pgsql", "DB_HOST": "db", "DB_PORT": "5432",
			"DB_DATABASE": "app", "DB_USERNAME": "app", "DB_PASSWORD": "devherd",
			"DATABASE_URL": "postgres://app:devherd@db:5432/app",
		}
		return svc, appEnv, true

	case "mongodb":
		svc := Service{
			Name:    "db",
			Image:   "mongo:7",
			Volumes: []string{"db_data:/data/db"},
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "app",
				"MONGO_INITDB_ROOT_PASSWORD": "devherd",
				"MONGO_INITDB_DATABASE":      "app",
			},
			ContainerPort: 27017,
			Backing:       true,
		}
		appEnv := map[string]string{
			"DB_HOST": "db", "DB_PORT": "27017",
			"MONGO_URL": "mongodb://app:devherd@db:27017/app?authSource=admin",
		}
		return svc, appEnv, true
	}

	return Service{}, nil, false
}

// AssignHostPorts asigna puertos de host LIBRES a los servicios publicables,
// evitando colisiones con lo que ya esté escuchando en el host.
func (p *Plan) AssignHostPorts() {
	used := map[int]bool{}
	for i := range p.Services {
		svc := &p.Services[i]
		if !svc.Publish || svc.ContainerPort == 0 {
			continue
		}
		port := freePort(svc.ContainerPort, used)
		svc.HostPort = port
		if port != 0 {
			used[port] = true
		}
	}
}

func freePort(preferred int, used map[int]bool) int {
	for p := preferred; p < preferred+200; p++ {
		if used[p] {
			continue
		}
		if isPortFree(p) {
			return p
		}
	}
	return 0
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// --- render ---

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
		if len(svc.Volumes) > 0 {
			b.WriteString("    volumes:\n")
			for _, v := range svc.Volumes {
				fmt.Fprintf(&b, "      - %s\n", v)
			}
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
		if len(svc.DependsOn) > 0 {
			b.WriteString("    depends_on:\n")
			for _, d := range svc.DependsOn {
				fmt.Fprintf(&b, "      - %s\n", d)
			}
		}
		if svc.Publish && svc.HostPort != 0 {
			b.WriteString("    ports:\n")
			fmt.Fprintf(&b, "      - \"%d:%d\"\n", svc.HostPort, svc.ContainerPort)
		}
	}

	if vols := namedVolumes(plan); len(vols) > 0 {
		b.WriteString("\nvolumes:\n")
		for _, v := range vols {
			fmt.Fprintf(&b, "  %s:\n", v)
		}
	}

	return b.String()
}

// RenderManifest genera el .devherd.yml que apunta al compose gestionado.
func RenderManifest(plan Plan) string {
	var b strings.Builder
	b.WriteString("# Generado por DevHerd (devherd scaffold).\n")
	b.WriteString("version: 1\n")
	b.WriteString("compose:\n")
	b.WriteString("  files:\n")
	fmt.Fprintf(&b, "    - %s\n", ManagedComposeFile)
	return b.String()
}

func namedVolumes(plan Plan) []string {
	seen := map[string]bool{}
	var names []string
	for _, svc := range plan.Services {
		for _, v := range svc.Volumes {
			name, _, ok := strings.Cut(v, ":")
			if !ok || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "/") {
				continue
			}
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// Write escribe el compose y el manifiesto (no sobrescribe salvo force).
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
		state, err := writeManaged(filepath.Join(plan.Root, f.name), f.content, force)
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
	return fileExists(filepath.Join(dir, "app.py")) && fileContains(filepath.Join(dir, "app.py"), "flask")
}

func isVueDir(dir string) bool {
	return packageJSONHasDependency(filepath.Join(dir, "package.json"), "vue")
}

func isNodeDir(dir string) bool {
	return fileExists(filepath.Join(dir, "package.json"))
}

func isGoDir(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod"))
}

func isLaravelDir(dir string) bool {
	return fileExists(filepath.Join(dir, "artisan")) && fileExists(filepath.Join(dir, "composer.json"))
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
	raw, ok := readPackageJSON(path)
	if !ok {
		return false
	}
	if _, ok := raw.Dependencies[dep]; ok {
		return true
	}
	_, ok = raw.DevDependencies[dep]
	return ok
}

func packageJSONHasScript(path, script string) bool {
	raw, ok := readPackageJSON(path)
	if !ok {
		return false
	}
	_, ok = raw.Scripts[script]
	return ok
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func readPackageJSON(path string) (packageJSON, bool) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return packageJSON{}, false
	}
	var raw packageJSON
	if err := json.Unmarshal(payload, &raw); err != nil {
		return packageJSON{}, false
	}
	return raw, true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
