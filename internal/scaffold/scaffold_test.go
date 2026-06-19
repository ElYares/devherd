package scaffold

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func vueFlaskFixture(t *testing.T) string {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "backend", "requirements.txt"), "flask==3.0.0\n")
	mustWrite(t, filepath.Join(root, "backend", "app.py"), "from flask import Flask\n")
	mustWrite(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"vue":"^3.4.0"}}`)
	return root
}

func TestDetectVueFlask(t *testing.T) {
	plan, err := Detect(vueFlaskFixture(t))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan.Framework != "vue+flask" || len(plan.Services) != 2 {
		t.Fatalf("got framework=%q services=%d", plan.Framework, len(plan.Services))
	}
}

func TestDetectSingleStacks(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		framework string
		image     string
	}{
		{"go", map[string]string{"go.mod": "module x\n"}, "go", "golang:1.25"},
		{"laravel", map[string]string{"artisan": "#!/usr/bin/env php\n", "composer.json": "{}"}, "laravel", "php:8.3-cli"},
		{"node", map[string]string{"package.json": `{"scripts":{"start":"node ."}}`}, "node", "node:20-alpine"},
		{"flask", map[string]string{"requirements.txt": "flask\n", "app.py": "from flask import Flask\n"}, "flask", "python:3.12-slim"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for f, c := range tc.files {
				mustWrite(t, filepath.Join(root, f), c)
			}
			plan, err := Detect(root)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if plan.Framework != tc.framework {
				t.Errorf("framework = %q, want %q", plan.Framework, tc.framework)
			}
			if plan.Services[0].Image != tc.image {
				t.Errorf("image = %q, want %q", plan.Services[0].Image, tc.image)
			}
		})
	}
}

func TestDetectUnsupportedLayout(t *testing.T) {
	if _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("expected error for unsupported layout")
	}
}

func TestAddDatabaseWiresAppEnvAndDependsOn(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	if err := plan.AddDatabase("postgres"); err != nil {
		t.Fatalf("AddDatabase: %v", err)
	}

	var dbFound bool
	for _, s := range plan.Services {
		if s.Name == "db" {
			dbFound = true
			if s.Image != "postgres:16" || !s.Backing {
				t.Errorf("db service = %+v", s)
			}
		}
	}
	if !dbFound {
		t.Fatal("db service not added")
	}

	for _, s := range plan.Services {
		if s.Backing {
			continue
		}
		if s.Env["DB_HOST"] != "db" {
			t.Errorf("app %q missing DB_HOST=db", s.Name)
		}
		if !contains(s.DependsOn, "db") {
			t.Errorf("app %q missing depends_on db", s.Name)
		}
	}
}

func TestAddDatabaseRejectsUnknown(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	if err := plan.AddDatabase("oracle"); err == nil {
		t.Fatal("expected error for unsupported db")
	}
	if err := plan.AddDatabase("none"); err != nil {
		t.Errorf("none should be a no-op, got %v", err)
	}
}

func TestAddRedisWiresEnv(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	plan.AddRedis()
	for _, s := range plan.Services {
		if s.Backing {
			continue
		}
		if s.Env["REDIS_HOST"] != "redis" {
			t.Errorf("app %q missing REDIS_HOST", s.Name)
		}
	}
}

func TestAssignHostPortsAvoidsCollisions(t *testing.T) {
	// Ocupa el puerto preferido (8000) y verifica que se asigne otro.
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(8000)))
	if err != nil {
		t.Skip("no se pudo ocupar 8000 para la prueba")
	}
	defer ln.Close()

	plan := Plan{Services: []Service{{Name: "app", ContainerPort: 8000, Publish: true}}}
	plan.AssignHostPorts()
	if plan.Services[0].HostPort == 8000 {
		t.Errorf("debería evitar el puerto 8000 ocupado, asignó %d", plan.Services[0].HostPort)
	}
	if plan.Services[0].HostPort == 0 {
		t.Error("no asignó ningún puerto libre")
	}
}

func TestBackingServicesAreNotPublished(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	_ = plan.AddDatabase("mysql")
	plan.AddRedis()
	plan.AssignHostPorts()

	yaml := RenderCompose(plan)
	// db y redis no deben publicar puertos ni colisionar; named volume presente.
	if strings.Contains(yaml, "3306:3306") || strings.Contains(yaml, "6379:6379") {
		t.Errorf("servicios de respaldo no deberían publicar puertos:\n%s", yaml)
	}
	if !strings.Contains(yaml, "volumes:\n  db_data:") {
		t.Errorf("falta el named volume db_data:\n%s", yaml)
	}
}

func TestWriteIsNonDestructive(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	plan.AssignHostPorts()

	states, err := Write(plan, false)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if states[ManagedComposeFile] != "created" || states[ManifestFile] != "created" {
		t.Errorf("states = %+v", states)
	}

	states2, _ := Write(plan, false)
	if !strings.Contains(states2[ManagedComposeFile], "skipped") {
		t.Errorf("second write = %q, want skipped", states2[ManagedComposeFile])
	}
}

func TestDetectPortAndScriptFromRepo(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "package.json"), `{"scripts":{"dev":"vite"},"dependencies":{}}`)
	mustWrite(t, filepath.Join(root, ".env"), "PORT=4000\n# comment\n")

	plan, err := Detect(root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	svc := plan.Services[0]
	if svc.ContainerPort != 4000 {
		t.Errorf("port = %d, want 4000 (del .env)", svc.ContainerPort)
	}
	if !strings.Contains(svc.Command, "npm run dev") {
		t.Errorf("command = %q, want script dev", svc.Command)
	}
}

func TestManifestDeclaresProxyForSingleStack(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module x\n")
	plan, _ := Detect(root)

	manifest := RenderManifest(plan)
	if !strings.Contains(manifest, "proxy:") || !strings.Contains(manifest, "service: app") || !strings.Contains(manifest, "port: 8080") {
		t.Errorf("manifest single-stack debe declarar proxy:\n%s", manifest)
	}
}

func TestManifestOmitsProxyForVueFlask(t *testing.T) {
	plan, _ := Detect(vueFlaskFixture(t))
	if strings.Contains(RenderManifest(plan), "proxy:") {
		t.Errorf("vue+flask no debe declarar proxy (lo resuelve el framework):\n%s", RenderManifest(plan))
	}
}

func laravelFixture(t *testing.T, env string, extra map[string]string) string {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "artisan"), "#!/usr/bin/env php\n")
	mustWrite(t, filepath.Join(root, "composer.json"), "{}")
	if env != "" {
		mustWrite(t, filepath.Join(root, ".env.example"), env)
	}
	for f, c := range extra {
		mustWrite(t, filepath.Join(root, f), c)
	}
	return root
}

func TestLaravelPlanIsCompleteWithBacking(t *testing.T) {
	root := laravelFixture(t, "DB_CONNECTION=mysql\nDB_DATABASE=cvboost\nDB_USERNAME=root\nDB_PASSWORD=\n",
		map[string]string{"vite.config.js": "export default {}\n", "package.json": `{"scripts":{"dev":"vite"}}`})

	plan, err := Detect(root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !plan.Complete {
		t.Error("laravel plan debería ser Complete")
	}

	names := map[string]Service{}
	for _, s := range plan.Services {
		names[s.Name] = s
	}
	for _, want := range []string{"app", "vite", "db", "redis"} {
		if _, ok := names[want]; !ok {
			t.Errorf("falta servicio %q (tengo %v)", want, names)
		}
	}

	app := names["app"]
	for _, frag := range []string{"composer install", "key:generate", "artisan migrate", "pdo_mysql", "docker-php-ext-install", "artisan serve"} {
		if !strings.Contains(app.Command, frag) {
			t.Errorf("comando laravel sin %q", frag)
		}
	}
	// idempotencia: no debe reinstalar redis a ciegas (debe consultar php -m)
	if !strings.Contains(app.Command, "php -m | grep -qix redis") {
		t.Errorf("comando laravel debería instalar redis solo si falta")
	}
	if app.Env["DB_DATABASE"] != "cvboost" || app.Env["DB_HOST"] != "db" {
		t.Errorf("app env DB mal cableado: %+v", app.Env)
	}
	// root + password vacío → MYSQL_ALLOW_EMPTY_PASSWORD
	if names["db"].Env["MYSQL_ALLOW_EMPTY_PASSWORD"] != "yes" || names["db"].Env["MYSQL_DATABASE"] != "cvboost" {
		t.Errorf("db env mal derivado del .env: %+v", names["db"].Env)
	}
}

func TestRenderEscapesDollarForCompose(t *testing.T) {
	root := laravelFixture(t, "DB_CONNECTION=mysql\n", nil)
	plan, _ := Detect(root)
	yaml := RenderCompose(plan)

	// el for-loop usa $e; debe salir como $$e para que compose no lo interpole
	if strings.Contains(yaml, `"$e"`) || !strings.Contains(yaml, `$$e`) {
		t.Errorf("el $e del comando debe escaparse a $$e en el YAML:\n%s", yaml)
	}
}

func TestLaravelDetectsPostgresFromEnv(t *testing.T) {
	root := laravelFixture(t, "DB_CONNECTION=pgsql\nDB_DATABASE=app\nDB_USERNAME=app\nDB_PASSWORD=secret\n", nil)
	plan, _ := Detect(root)

	var db Service
	for _, s := range plan.Services {
		if s.Name == "db" {
			db = s
		}
	}
	if db.Image != "postgres:16" {
		t.Errorf("db image = %q, want postgres:16", db.Image)
	}
	if !strings.Contains(plan.Services[0].Command, "pdo_pgsql") {
		t.Errorf("comando laravel postgres debe instalar pdo_pgsql")
	}
}

func TestLaravelCompletePlanRendersValidStructure(t *testing.T) {
	root := laravelFixture(t, "DB_CONNECTION=mysql\nDB_DATABASE=app\nDB_USERNAME=app\nDB_PASSWORD=secret\n", nil)
	plan, _ := Detect(root)
	plan.AssignHostPorts()

	yaml := RenderCompose(plan)
	// el comando va en forma exec (lista) por las comillas internas
	if !strings.Contains(yaml, `command: ["sh", "-c",`) {
		t.Errorf("comando debería renderizarse en forma exec:\n%s", yaml)
	}
	// db (mysql user app) usa MYSQL_USER, no debe publicar puerto
	if strings.Contains(yaml, "3306:3306") {
		t.Errorf("la db no debe publicar puerto:\n%s", yaml)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
