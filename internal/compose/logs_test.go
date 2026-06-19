package compose

import (
	"reflect"
	"testing"
)

func TestLogsArgsDefault(t *testing.T) {
	project := Project{
		Root:         "/tmp/app",
		ProjectName:  "devherd-app",
		ComposeFiles: []string{"/tmp/app/docker-compose.yml"},
	}

	got := LogsArgs(project, LogsOptions{})
	want := []string{
		"compose", "--project-name", "devherd-app",
		"-f", "/tmp/app/docker-compose.yml",
		"logs",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LogsArgs() = %v, want %v", got, want)
	}
}

func TestLogsArgsWithFollowTailAndServices(t *testing.T) {
	project := Project{
		Root:         "/tmp/app",
		ProjectName:  "devherd-app",
		EnvFile:      "/tmp/app/.env",
		ComposeFiles: []string{"/tmp/app/docker-compose.yml", "/tmp/app/override.yml"},
	}

	got := LogsArgs(project, LogsOptions{Follow: true, Tail: "100", Services: []string{"web", "api"}})
	want := []string{
		"compose", "--project-name", "devherd-app",
		"--env-file", "/tmp/app/.env",
		"-f", "/tmp/app/docker-compose.yml",
		"-f", "/tmp/app/override.yml",
		"logs", "--follow", "--tail", "100", "web", "api",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LogsArgs() = %v, want %v", got, want)
	}
}
