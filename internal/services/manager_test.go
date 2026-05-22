package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/config"
)

func TestBootstrapWritesSharedServicesCompose(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(config.Paths{ComposeDir: dir})

	if err := manager.bootstrap(); err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(dir, stackDir, composeFile))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}

	content := string(payload)
	for _, fragment := range []string{
		"container_name: infra_redis",
		"127.0.0.1:6379:6379",
		"infra_net:",
		"- redis",
	} {
		if !strings.Contains(content, fragment) {
			t.Fatalf("expected fragment %q in compose:\n%s", fragment, content)
		}
	}
}

func TestValidateServiceRejectsUnknownService(t *testing.T) {
	err := validateService("postgres")
	if err == nil {
		t.Fatal("expected unsupported service error")
	}

	if !strings.Contains(err.Error(), "redis") || !strings.Contains(err.Error(), "mailpit") {
		t.Fatalf("expected supported service list in error, got %q", err.Error())
	}
}
