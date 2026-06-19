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

// Service es un servicio del compose generado. Command es el script de shell
// (sin envoltura sh -c), que se renderiza en forma exec ["sh","-c",script].
type Service struct {
	Name          string
	Image         string
	WorkingDir    string
	Volumes       []string // "./backend:/app" (bind) o "db_data:/var/lib/mysql" (named)
	Command       string
	Env           map[string]string
	DependsOn     []string
	ContainerPort int
	HostPort      int
	Publish       bool
	Backing       bool
}

// Plan es el resultado de analizar un proyecto. Complete indica que la detección
// ya incluyó los servicios de respaldo (caso Laravel, que los deriva del .env).
type Plan struct {
	Root      string
	Framework string
	Complete  bool
	Services  []Service
}

// SupportedDatabases lista las opciones de base de datos del menú.
var SupportedDatabases = []string{"mysql", "mariadb", "postgres", "mongodb", "none"}

// Detect analiza el proyecto y produce un Plan.
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
					// Puertos fijos: contrato de proxy.BuildExternalProject para vue+flask.
					flaskService("backend", filepath.Base(backend), 8000),
					vueService("frontend", filepath.Base(frontend), 5173, "dev"),
				},
			}, nil
		}
	}

	// 2) Stacks de un solo servicio en la raíz (puerto/comando detectados del repo).
	switch {
	case isLaravelDir(abs):
		return laravelPlan(abs), nil
	case isVueDir(abs):
		return single(abs, "vue", vueService("app", ".", detectPort(abs, 5173, "PORT"), pickScript(abs, "dev", "serve", "start"))), nil
	case isFlaskDir(abs):
		return single(abs, "flask", flaskService("app", ".", detectPort(abs, 8000, "FLASK_RUN_PORT", "PORT"))), nil
	case isNodeDir(abs):
		return single(abs, "node", nodeService(abs, detectPort(abs, 3000, "PORT"), pickScript(abs, "dev", "start"))), nil
	case isGoDir(abs):
		return single(abs, "go", goService(detectPort(abs, 8080, "PORT"))), nil
	}

	return Plan{}, fmt.Errorf("scaffold: no reconozco el stack en %s (soportados: vue+flask, laravel, vue, flask, node, go)", abs)
}

func single(root, framework string, svc Service) Plan {
	return Plan{Root: root, Framework: framework, Services: []Service{svc}}
}

// --- servicios de aplicación por stack ---

func flaskService(name, dir string, port int) Service {
	return Service{
		Name:          name,
		Image:         "python:3.12-slim",
		WorkingDir:    "/app",
		Volumes:       []string{mount(dir)},
		Command:       fmt.Sprintf("pip install -r requirements.txt && flask run --host 0.0.0.0 --port %d", port),
		Env:           map[string]string{"FLASK_APP": "app.py", "FLASK_RUN_PORT": strconv.Itoa(port)},
		ContainerPort: port,
		Publish:       true,
	}
}

func vueService(name, dir string, port int, script string) Service {
	return Service{
		Name:          name,
		Image:         "node:20-alpine",
		WorkingDir:    "/app",
		Volumes:       []string{mount(dir)},
		Command:       fmt.Sprintf("npm install && npm run %s -- --host 0.0.0.0 --port %d", script, port),
		ContainerPort: port,
		Publish:       true,
	}
}

func nodeService(dir string, port int, script string) Service {
	return Service{
		Name:          "app",
		Image:         "node:20-alpine",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       fmt.Sprintf("npm install && npm run %s", script),
		Env:           map[string]string{"PORT": strconv.Itoa(port)},
		ContainerPort: port,
		Publish:       true,
	}
}

func goService(port int) Service {
	return Service{
		Name:          "app",
		Image:         "golang:1.25",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       "go run ./...",
		ContainerPort: port,
		Publish:       true,
	}
}

func viteService() Service {
	return Service{
		Name:          "vite",
		Image:         "node:20-alpine",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       "npm install && npm run dev -- --host 0.0.0.0 --port 5173",
		ContainerPort: 5173,
		Publish:       true,
	}
}

func mount(dir string) string {
	if dir == "." || dir == "" {
		return "./:/app"
	}
	return "./" + dir + ":/app"
}

// --- Laravel (plan completo: app php + vite + db del .env + redis) ---

func laravelPlan(root string) Plan {
	env := projectEnv(root)
	dbKind := laravelDBKind(env["DB_CONNECTION"])
	port := detectPort(root, 8000, "APP_PORT", "PORT")

	app := Service{
		Name:          "app",
		Image:         "php:8.3-cli",
		WorkingDir:    "/app",
		Volumes:       []string{mount(".")},
		Command:       laravelCommand(dbKind, port),
		Env:           map[string]string{},
		ContainerPort: port,
		Publish:       true,
	}

	plan := Plan{Root: root, Framework: "laravel", Complete: true, Services: []Service{app}}

	if hasVite(root) {
		plan.Services = append(plan.Services, viteService())
	}

	if dbKind != "none" {
		db, appEnv := laravelDBService(dbKind, env)
		plan.Services = append(plan.Services, db)
		for k, v := range appEnv {
			plan.Services[0].Env[k] = v
		}
		plan.Services[0].DependsOn = append(plan.Services[0].DependsOn, "db")
	}

	plan.Services = append(plan.Services, Service{Name: "redis", Image: "redis:7-alpine", ContainerPort: 6379, Backing: true})
	plan.Services[0].Env["REDIS_HOST"] = "redis"
	plan.Services[0].Env["REDIS_PORT"] = "6379"
	plan.Services[0].DependsOn = append(plan.Services[0].DependsOn, "redis")

	return plan
}

func laravelDBKind(connection string) string {
	switch strings.ToLower(strings.TrimSpace(connection)) {
	case "pgsql", "postgres", "postgresql":
		return "postgres"
	case "sqlite":
		return "none"
	case "mariadb":
		return "mariadb"
	default:
		return "mysql"
	}
}

// laravelCommand arma el arranque sin Dockerfile: extensiones + composer + .env +
// key + migrate (esperando a la DB) + artisan serve. Las extensiones dependen
// del motor de base de datos.
func laravelCommand(dbKind string, port int) string {
	ext := "pdo_mysql"
	libs := "libzip-dev libpng-dev libicu-dev unzip git"
	if dbKind == "postgres" {
		ext = "pdo_pgsql"
		libs += " libpq-dev"
	}

	steps := []string{
		"apt-get update",
		"apt-get install -y -q " + libs,
		"docker-php-ext-install " + ext + " zip gd bcmath intl",
		"(pecl install redis && docker-php-ext-enable redis)",
		"curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer",
		"composer install --no-interaction --no-progress",
		"([ -f .env ] || cp .env.example .env)",
		"php artisan key:generate --force",
		"until php artisan migrate --force; do echo 'esperando a la base de datos...'; sleep 3; done",
		fmt.Sprintf("php artisan serve --host 0.0.0.0 --port %d", port),
	}
	return strings.Join(steps, " && ")
}

func laravelDBService(kind string, env map[string]string) (Service, map[string]string) {
	name := orDefault(env["DB_DATABASE"], "app")
	user := orDefault(env["DB_USERNAME"], "app")
	pass := env["DB_PASSWORD"]

	if kind == "postgres" {
		if pass == "" {
			pass = "secret"
		}
		svc := Service{
			Name:  "db",
			Image: "postgres:16",
			Env: map[string]string{
				"POSTGRES_DB": name, "POSTGRES_USER": user, "POSTGRES_PASSWORD": pass,
			},
			Volumes:       []string{"db_data:/var/lib/postgresql/data"},
			ContainerPort: 5432,
			Backing:       true,
		}
		appEnv := map[string]string{
			"DB_CONNECTION": "pgsql", "DB_HOST": "db", "DB_PORT": "5432",
			"DB_DATABASE": name, "DB_USERNAME": user, "DB_PASSWORD": pass,
		}
		return svc, appEnv
	}

	image := "mysql:8"
	if kind == "mariadb" {
		image = "mariadb:11"
	}
	dbEnv := map[string]string{"MYSQL_DATABASE": name}
	if user == "root" {
		if pass == "" {
			dbEnv["MYSQL_ALLOW_EMPTY_PASSWORD"] = "yes"
		} else {
			dbEnv["MYSQL_ROOT_PASSWORD"] = pass
		}
	} else {
		dbEnv["MYSQL_ROOT_PASSWORD"] = orDefault(pass, "secret")
		dbEnv["MYSQL_USER"] = user
		dbEnv["MYSQL_PASSWORD"] = pass
	}
	svc := Service{
		Name:          "db",
		Image:         image,
		Env:           dbEnv,
		Volumes:       []string{"db_data:/var/lib/mysql"},
		ContainerPort: 3306,
		Backing:       true,
	}
	appEnv := map[string]string{
		"DB_CONNECTION": "mysql", "DB_HOST": "db", "DB_PORT": "3306",
		"DB_DATABASE": name, "DB_USERNAME": user, "DB_PASSWORD": pass,
	}
	return svc, appEnv
}

func hasVite(root string) bool {
	return fileExists(filepath.Join(root, "vite.config.js")) ||
		fileExists(filepath.Join(root, "vite.config.ts")) ||
		fileExists(filepath.Join(root, "package.json"))
}

// --- servicios de respaldo (stacks no-Laravel, vía flags/menú) ---

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
				"MYSQL_ROOT_PASSWORD": "devherd", "MYSQL_DATABASE": "app",
				"MYSQL_USER": "app", "MYSQL_PASSWORD": "devherd",
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
			// Forma exec con script en sh -c; %q evita problemas de comillas/escapes.
			fmt.Fprintf(&b, "    command: [\"sh\", \"-c\", %q]\n", svc.Command)
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

// RenderManifest genera el .devherd.yml. Para stacks de un solo servicio de app
// declara proxy.service/port (vue+flask lo resuelve el framework).
func RenderManifest(plan Plan) string {
	var b strings.Builder
	b.WriteString("# Generado por DevHerd (devherd scaffold).\n")
	b.WriteString("version: 1\n")
	b.WriteString("compose:\n")
	b.WriteString("  files:\n")
	fmt.Fprintf(&b, "    - %s\n", ManagedComposeFile)

	if svc, ok := primaryAppService(plan); ok {
		b.WriteString("proxy:\n")
		fmt.Fprintf(&b, "  service: %s\n", svc.Name)
		fmt.Fprintf(&b, "  port: %d\n", svc.ContainerPort)
	}

	return b.String()
}

// primaryAppService devuelve el servicio web principal para enrutar el proxy.
// En vue+flask no aplica (lo resuelve el framework). En Laravel es "app".
func primaryAppService(plan Plan) (Service, bool) {
	if plan.Framework == "vue+flask" {
		return Service{}, false
	}
	if plan.Framework == "laravel" {
		for _, s := range plan.Services {
			if s.Name == "app" {
				return s, true
			}
		}
	}
	var apps []Service
	for _, s := range plan.Services {
		if !s.Backing {
			apps = append(apps, s)
		}
	}
	if len(apps) == 1 {
		return apps[0], true
	}
	return Service{}, false
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

// detectPort devuelve el puerto declarado en el .env del repo o el default.
func detectPort(dir string, def int, keys ...string) int {
	env := projectEnv(dir)
	for _, k := range keys {
		if v, ok := env[k]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
		}
	}
	return def
}

// projectEnv lee .env.example y luego .env (el .env real tiene prioridad).
func projectEnv(dir string) map[string]string {
	env := readDotEnv(filepath.Join(dir, ".env.example"))
	for k, v := range readDotEnv(filepath.Join(dir, ".env")) {
		env[k] = v
	}
	return env
}

func readDotEnv(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

func pickScript(dir string, prefs ...string) string {
	for _, p := range prefs {
		if packageJSONHasScript(filepath.Join(dir, "package.json"), p) {
			return p
		}
	}
	return prefs[len(prefs)-1]
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
