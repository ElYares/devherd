package observe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/compose"
)

func TestBuildComposeOverrideObservesSelectedServices(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: nginx\n  worker:\n    image: alpine\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project := compose.Project{
		Root:         dir,
		ComposeFiles: []string{composeFile},
	}

	result, err := BuildComposeOverride(project, AttachOptions{
		ProjectName: "demo",
		Stack:       "laravel",
		Services:    []string{"web"},
		DSN:         "http://devherd@127.0.0.1:9777/demo",
		Environment: "local",
	})
	if err != nil {
		t.Fatalf("BuildComposeOverride returned error: %v", err)
	}

	if len(result.Services) != 1 || result.Services[0] != "web" {
		t.Fatalf("unexpected services: %#v", result.Services)
	}

	content := result.Content
	for _, fragment := range []string{
		"web:",
		"SENTRY_DSN: http://devherd@127.0.0.1:9777/demo",
		"SENTRY_ENVIRONMENT: local",
		"DEVHERD_OBSERVE: \"1\"",
		"devherd.project: demo",
		"devherd.service: web",
	} {
		if !strings.Contains(content, fragment) {
			t.Fatalf("expected fragment %q in override:\n%s", fragment, content)
		}
	}

	if strings.Contains(content, "worker:") {
		t.Fatalf("did not expect worker service in selected override:\n%s", content)
	}
}

func TestBuildComposeOverrideRejectsMissingService(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	_, err := BuildComposeOverride(compose.Project{
		Root:         dir,
		ComposeFiles: []string{composeFile},
	}, AttachOptions{
		ProjectName: "demo",
		Stack:       "node",
		Services:    []string{"worker"},
		DSN:         "http://devherd@127.0.0.1:9777/demo",
	})
	if err == nil {
		t.Fatalf("expected missing service error")
	}
}
