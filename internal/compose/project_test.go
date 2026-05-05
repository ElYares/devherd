package compose

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveProjectDefaultsToSingleComposeFile(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project, err := ResolveProject(dir)
	if err != nil {
		t.Fatalf("ResolveProject returned error: %v", err)
	}

	if project.Root != dir {
		t.Fatalf("unexpected root: got %q want %q", project.Root, dir)
	}

	if !reflect.DeepEqual(project.ComposeFiles, []string{composeFile}) {
		t.Fatalf("unexpected compose files: %#v", project.ComposeFiles)
	}

	if project.EnvFile != "" {
		t.Fatalf("unexpected env file: %q", project.EnvFile)
	}

	if project.Source != ProjectSourceAutodetect {
		t.Fatalf("unexpected source: got %q want %q", project.Source, ProjectSourceAutodetect)
	}
}

func TestResolveProjectUsesManifestComposeFiles(t *testing.T) {
	dir := t.TempDir()
	baseCompose := filepath.Join(dir, "docker-compose.yml")
	sharedCompose := filepath.Join(dir, "docker-compose.shared.yml")
	envFile := filepath.Join(dir, ".env.devherd")

	for _, path := range []string{baseCompose, sharedCompose, envFile} {
		if err := os.WriteFile(path, []byte("stub\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	manifestPath := filepath.Join(dir, manifestFileName)
	manifest := []byte("version: 1\ncompose:\n  files:\n    - docker-compose.yml\n    - docker-compose.shared.yml\n  env_file: .env.devherd\nproxy:\n  domain: aang.localhost\n  service: web\n  port: 80\n")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	project, err := ResolveProject(dir)
	if err != nil {
		t.Fatalf("ResolveProject returned error: %v", err)
	}

	wantFiles := []string{baseCompose, sharedCompose}
	if !reflect.DeepEqual(project.ComposeFiles, wantFiles) {
		t.Fatalf("unexpected compose files: got %#v want %#v", project.ComposeFiles, wantFiles)
	}

	if project.EnvFile != envFile {
		t.Fatalf("unexpected env file: got %q want %q", project.EnvFile, envFile)
	}

	if project.Source != ProjectSourceManifest {
		t.Fatalf("unexpected source: got %q want %q", project.Source, ProjectSourceManifest)
	}

	if project.Proxy.Domain != "aang.localhost" {
		t.Fatalf("unexpected proxy domain: got %q", project.Proxy.Domain)
	}

	if project.Proxy.Service != "web" || project.Proxy.Port != 80 {
		t.Fatalf("unexpected proxy config: %#v", project.Proxy)
	}
}

func TestResolveProjectErrorsWhenManifestComposeFileMissing(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, manifestFileName)
	manifest := []byte("version: 1\ncompose:\n  files:\n    - missing.yml\n")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := ResolveProject(dir)
	if err == nil {
		t.Fatalf("ResolveProject expected error, got nil")
	}
}

func TestComposeArgsIncludesEnvFileAndAllComposeFiles(t *testing.T) {
	project := Project{
		Root:         "/tmp/project",
		ComposeFiles: []string{"/tmp/project/docker-compose.yml", "/tmp/project/docker-compose.shared.yml"},
		EnvFile:      "/tmp/project/.env.devherd",
	}

	got := composeArgs(project)
	want := []string{
		"compose",
		"--env-file", "/tmp/project/.env.devherd",
		"-f", "/tmp/project/docker-compose.yml",
		"-f", "/tmp/project/docker-compose.shared.yml",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected compose args: got %#v want %#v", got, want)
	}
}

func TestPlanReturnsDockerCommand(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	project, command, err := Plan(dir)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if project.Root != dir {
		t.Fatalf("unexpected root: got %q want %q", project.Root, dir)
	}

	want := []string{"docker", "compose", "-f", composeFile}
	if !reflect.DeepEqual(command, want) {
		t.Fatalf("unexpected command: got %#v want %#v", command, want)
	}
}
