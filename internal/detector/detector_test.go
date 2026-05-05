package detector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectExampleProject(t *testing.T) {
	projectPath := filepath.Join("..", "..", "testdata", "projects", "hello-vue-flask-docker")

	project, ok, err := DetectProject(projectPath)
	if err != nil {
		t.Fatalf("detect project: %v", err)
	}

	if !ok {
		t.Fatal("expected example project to be detected")
	}

	if project.Framework != "vue+flask" {
		t.Fatalf("expected framework vue+flask, got %q", project.Framework)
	}

	if project.Stack != "node+python+docker" {
		t.Fatalf("expected node+python+docker stack, got %q", project.Stack)
	}
}

func TestDiscoverExamplesDirectory(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "projects")

	projects, err := Discover(root)
	if err != nil {
		t.Fatalf("discover examples: %v", err)
	}

	if len(projects) == 0 {
		t.Fatal("expected at least one detected project")
	}

	if projects[0].Name != "hello-vue-flask-docker" {
		t.Fatalf("expected hello-vue-flask-docker, got %q", projects[0].Name)
	}
}

func TestDiscoverSkipsNodeModulesDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"vue":"^3.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write root package.json: %v", err)
	}

	nodeModulesDir := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nodeModulesDir, 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeModulesDir, "package.json"), []byte(`{"dependencies":{"vue":"^3.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write nested package.json: %v", err)
	}

	projects, err := Discover(root)
	if err != nil {
		t.Fatalf("discover projects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected exactly one detected project, got %d: %#v", len(projects), projects)
	}

	if projects[0].Name != filepath.Base(root) {
		t.Fatalf("expected %s, got %q", filepath.Base(root), projects[0].Name)
	}
}

func TestDiscoverPrefersManagedRootOverNestedChildren(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".devherd.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte("services:\n  frontend:\n    image: nginx:alpine\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	frontendDir := filepath.Join(root, "frontend")
	if err := os.MkdirAll(frontendDir, 0o755); err != nil {
		t.Fatalf("mkdir frontend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "package.json"), []byte(`{"dependencies":{"vue":"^3.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write frontend package.json: %v", err)
	}

	backendDir := filepath.Join(root, "backend")
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backendDir, "requirements.txt"), []byte("flask==3.0.0\n"), 0o644); err != nil {
		t.Fatalf("write backend requirements.txt: %v", err)
	}

	projects, err := Discover(root)
	if err != nil {
		t.Fatalf("discover projects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected exactly one managed root project, got %d: %#v", len(projects), projects)
	}

	if projects[0].Path != root {
		t.Fatalf("expected root project path %s, got %s", root, projects[0].Path)
	}

	if projects[0].Framework != "vue+flask" {
		t.Fatalf("expected merged framework vue+flask, got %q", projects[0].Framework)
	}
}
