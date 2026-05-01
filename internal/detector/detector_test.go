package detector

import (
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
