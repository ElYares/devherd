package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVueFlaskFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backend := filepath.Join(root, "backend")
	frontend := filepath.Join(root, "frontend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(frontend, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(backend, "requirements.txt"), "flask==3.0.0\n")
	mustWrite(t, filepath.Join(backend, "app.py"), "from flask import Flask\napp = Flask(__name__)\n")
	mustWrite(t, filepath.Join(frontend, "package.json"), `{"name":"f","dependencies":{"vue":"^3.4.0"}}`)
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectVueFlask(t *testing.T) {
	plan, err := Detect(writeVueFlaskFixture(t))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan.Framework != "vue+flask" {
		t.Errorf("framework = %q, want vue+flask", plan.Framework)
	}
	if len(plan.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(plan.Services))
	}

	byName := map[string]Service{}
	for _, s := range plan.Services {
		byName[s.Name] = s
	}
	if byName["backend"].Image != "python:3.12-slim" {
		t.Errorf("backend image = %q", byName["backend"].Image)
	}
	if byName["frontend"].Image != "node:20-alpine" {
		t.Errorf("frontend image = %q", byName["frontend"].Image)
	}
}

func TestDetectUnsupportedLayout(t *testing.T) {
	if _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("expected error for unsupported layout")
	}
}

func TestRenderComposeContainsServicesPortsAndMounts(t *testing.T) {
	plan, _ := Detect(writeVueFlaskFixture(t))
	yaml := RenderCompose(plan)

	for _, want := range []string{
		"services:",
		"  backend:",
		"  frontend:",
		"image: python:3.12-slim",
		"image: node:20-alpine",
		"- ./backend:/app",
		"- ./frontend:/app",
		"flask run",
		"npm run dev",
		`"8000:8000"`,
		`"5173:5173"`,
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("compose missing %q:\n%s", want, yaml)
		}
	}
}

func TestWriteIsNonDestructive(t *testing.T) {
	root := writeVueFlaskFixture(t)
	plan, _ := Detect(root)

	states, err := Write(plan, false)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if states[ManagedComposeFile] != "created" {
		t.Errorf("compose state = %q, want created", states[ManagedComposeFile])
	}
	if states[ManifestFile] != "created" {
		t.Errorf("manifest state = %q, want created", states[ManifestFile])
	}

	// Segunda escritura sin force: no debe sobrescribir.
	states2, _ := Write(plan, false)
	if !strings.Contains(states2[ManagedComposeFile], "skipped") {
		t.Errorf("second write state = %q, want skipped", states2[ManagedComposeFile])
	}

	// El manifiesto apunta al compose generado.
	data, _ := os.ReadFile(filepath.Join(root, ManifestFile))
	if !strings.Contains(string(data), ManagedComposeFile) {
		t.Errorf("manifest does not reference compose:\n%s", data)
	}
}
