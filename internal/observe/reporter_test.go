package observe

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureReporterWritesTheLaravelFile(t *testing.T) {
	root := t.TempDir()

	result, err := EnsureReporter(root, "laravel", false)
	if err != nil {
		t.Fatalf("EnsureReporter returned error: %v", err)
	}

	want := filepath.Join(root, "app", "Exceptions", "DevherdObserveReporter.php")
	if result.Path != want {
		t.Errorf("path = %q, want %q", result.Path, want)
	}
	if result.Wiring == "" {
		t.Error("the result must explain how to wire the reporter up")
	}

	payload, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read written reporter: %v", err)
	}

	content := string(payload)
	for _, want := range []string{
		"namespace App\\Exceptions;",
		"class DevherdObserveReporter",
		"public static function report(",
		"public static function capture(",
		"'fingerprint'",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("written reporter is missing %q", want)
		}
	}
}

func TestEnsureReporterKeepsExistingFile(t *testing.T) {
	// El reporter es codigo del proyecto y puede estar editado a mano: pisarlo
	// en silencio seria perder trabajo del usuario.
	root := t.TempDir()
	path := filepath.Join(root, "app", "Exceptions", "DevherdObserveReporter.php")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("prepare project: %v", err)
	}
	if err := os.WriteFile(path, []byte("<?php // mio"), 0o644); err != nil {
		t.Fatalf("write existing reporter: %v", err)
	}

	result, err := EnsureReporter(root, "laravel", false)
	if !errors.Is(err, ErrReporterExists) {
		t.Fatalf("err = %v, want ErrReporterExists", err)
	}
	if result.Path != path {
		t.Errorf("path = %q, want the existing one", result.Path)
	}

	payload, _ := os.ReadFile(path)
	if string(payload) != "<?php // mio" {
		t.Error("the existing reporter was overwritten")
	}
}

func TestEnsureReporterOverwritesWithForce(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app", "Exceptions", "DevherdObserveReporter.php")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("prepare project: %v", err)
	}
	if err := os.WriteFile(path, []byte("<?php // viejo"), 0o644); err != nil {
		t.Fatalf("write existing reporter: %v", err)
	}

	if _, err := EnsureReporter(root, "laravel", true); err != nil {
		t.Fatalf("EnsureReporter returned error: %v", err)
	}

	payload, _ := os.ReadFile(path)
	if strings.Contains(string(payload), "viejo") {
		t.Error("--force must replace the file")
	}
}

func TestEnsureReporterRejectsUnsupportedStack(t *testing.T) {
	_, err := EnsureReporter(t.TempDir(), "node", false)
	if err == nil {
		t.Fatal("expected an error for a stack without reporter")
	}
	if !strings.Contains(err.Error(), "laravel") {
		t.Errorf("error %q should list the supported stacks", err)
	}
}

func TestEnsureReporterRequiresRoot(t *testing.T) {
	if _, err := EnsureReporter("  ", "laravel", false); err == nil {
		t.Fatal("expected an error without a project root")
	}
}
