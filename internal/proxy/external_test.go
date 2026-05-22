package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
)

func TestBuildExternalProjectUsesManifestProxy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  web:\n    image: nginx:alpine\n")
	writeTestFile(t, filepath.Join(dir, ".devherd.yml"), "version: 1\ncompose:\n  files:\n    - docker-compose.yml\nproxy:\n  domain: aang.localhost\n  service: web\n  port: 80\n")

	project, err := BuildExternalProject(config.Default(), database.ProjectRecord{
		Name: "aang-server",
		Path: dir,
	})
	if err != nil {
		t.Fatalf("BuildExternalProject returned error: %v", err)
	}

	if project.Domain != "aang.localhost" {
		t.Fatalf("unexpected domain: %q", project.Domain)
	}

	if len(project.Aliases) != 1 || project.Aliases[0].Name != "aang-web" {
		t.Fatalf("unexpected aliases: %#v", project.Aliases)
	}

	if len(project.Routes) != 1 || project.Routes[0].Target != "aang-web:80" {
		t.Fatalf("unexpected routes: %#v", project.Routes)
	}
}

func TestBuildExternalProjectUsesVueFlaskFallback(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "docker-compose.yml"), "services:\n  backend:\n    image: python:3.12\n  frontend:\n    image: node:20\n")

	project, err := BuildExternalProject(config.Config{
		LocalTLD: "localhost",
	}, database.ProjectRecord{
		Name:      "hello-vue-flask-docker",
		Path:      dir,
		Domain:    "mi-demo.localhost",
		Framework: "vue+flask",
	})
	if err != nil {
		t.Fatalf("BuildExternalProject returned error: %v", err)
	}

	if len(project.Aliases) != 2 {
		t.Fatalf("unexpected alias count: %#v", project.Aliases)
	}

	if project.Routes[0].Target != "mi-demo-backend:8000" {
		t.Fatalf("unexpected backend route: %#v", project.Routes[0])
	}

	if project.Routes[1].Target != "mi-demo-frontend:5173" {
		t.Fatalf("unexpected frontend route: %#v", project.Routes[1])
	}
}

func TestEnsureComposeOverrideWritesAliases(t *testing.T) {
	dir := t.TempDir()
	overridePath, err := EnsureComposeOverride(config.Config{
		Proxy: config.ProxyConfig{
			ExternalNetwork: "shared_proxy",
		},
	}, ExternalProject{
		Compose: compose.Project{Root: dir},
		Aliases: []Alias{
			{Service: "backend", Name: "mi-demo-backend"},
			{Service: "frontend", Name: "mi-demo-frontend"},
		},
	})
	if err != nil {
		t.Fatalf("EnsureComposeOverride returned error: %v", err)
	}

	payload, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read override: %v", err)
	}

	content := string(payload)
	for _, fragment := range []string{
		"backend:",
		"frontend:",
		"shared_proxy:",
		"mi-demo-backend",
		"mi-demo-frontend",
	} {
		if !strings.Contains(content, fragment) {
			t.Fatalf("expected fragment %q in override\n%s", fragment, content)
		}
	}
}

func TestStripManagedDomainsReplacesSelectedSites(t *testing.T) {
	original := `{
	auto_https off
}

http://aang.localhost {
	reverse_proxy aang_web:80
}

http://other.localhost {
	reverse_proxy other_web:80
}`

	updated := stripManagedDomains(original, []string{"aang.localhost"})
	if strings.Contains(updated, "aang.localhost") {
		t.Fatalf("expected aang.localhost block to be removed\n%s", updated)
	}

	if !strings.Contains(updated, "other.localhost") {
		t.Fatalf("expected unrelated block to remain\n%s", updated)
	}
}

func TestBootstrapExternalProxyCreatesAndReusesFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Proxy: config.ProxyConfig{
			ExternalDir:           dir,
			ExternalNetwork:       "portable_net",
			ExternalContainerName: "portable_caddy",
		},
	}

	first, err := BootstrapExternalProxy(cfg)
	if err != nil {
		t.Fatalf("BootstrapExternalProxy returned error: %v", err)
	}

	if first.ComposeFileStatus != "created" || first.CaddyfileStatus != "created" || first.EnvFileStatus != "created" {
		t.Fatalf("expected created statuses on first bootstrap: %#v", first)
	}

	composePayload, err := os.ReadFile(filepath.Join(dir, ExternalProxyComposeFile))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	if !strings.Contains(string(composePayload), "portable_caddy") || !strings.Contains(string(composePayload), "portable_net") {
		t.Fatalf("unexpected compose template output:\n%s", string(composePayload))
	}

	second, err := BootstrapExternalProxy(cfg)
	if err != nil {
		t.Fatalf("BootstrapExternalProxy second run returned error: %v", err)
	}

	if second.ComposeFileStatus != "reused" || second.CaddyfileStatus != "reused" || second.EnvFileStatus != "reused" {
		t.Fatalf("expected reused statuses on second bootstrap: %#v", second)
	}
}

func TestBootstrapExternalProxyWithForceUpdatesManagedFilesButPreservesEnv(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Proxy: config.ProxyConfig{
			ExternalDir:           dir,
			ExternalNetwork:       "portable_net",
			ExternalContainerName: "portable_caddy",
		},
	}

	if _, err := BootstrapExternalProxy(cfg); err != nil {
		t.Fatalf("BootstrapExternalProxy returned error: %v", err)
	}

	writeTestFile(t, filepath.Join(dir, ExternalProxyComposeFile), "services:\n  broken:\n")
	writeTestFile(t, filepath.Join(dir, ExternalProxyCaddyfile), "broken.localhost {\n\treverse_proxy broken:80\n}\n")
	writeTestFile(t, filepath.Join(dir, ExternalProxyEnvFile), "CADDY_IMAGE=custom:caddy\n")
	writeTestFile(t, filepath.Join(dir, ".env.example"), "CADDY_IMAGE=broken:caddy\n")

	result, err := BootstrapExternalProxyWithOptions(cfg, BootstrapOptions{Force: true})
	if err != nil {
		t.Fatalf("BootstrapExternalProxyWithOptions returned error: %v", err)
	}

	if result.ComposeFileStatus != "updated" || result.CaddyfileStatus != "updated" || result.EnvExampleStatus != "updated" {
		t.Fatalf("expected updated statuses for managed templates: %#v", result)
	}
	if result.EnvFileStatus != "reused" {
		t.Fatalf("expected .env to be preserved, got %#v", result)
	}

	composePayload, err := os.ReadFile(filepath.Join(dir, ExternalProxyComposeFile))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	if !strings.Contains(string(composePayload), "portable_caddy") || !strings.Contains(string(composePayload), "portable_net") {
		t.Fatalf("expected forced bootstrap to restore managed compose template:\n%s", string(composePayload))
	}

	envPayload, err := os.ReadFile(filepath.Join(dir, ExternalProxyEnvFile))
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if string(envPayload) != "CADDY_IMAGE=custom:caddy\n" {
		t.Fatalf("expected .env to remain unchanged, got:\n%s", string(envPayload))
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
