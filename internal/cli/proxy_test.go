package cli

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/proxy"
)

func TestSyncManagedDomainsUsesCollectedDomains(t *testing.T) {
	original := syncHosts
	t.Cleanup(func() {
		syncHosts = original
	})

	var got []string
	syncHosts = func(domains []string) error {
		got = append([]string(nil), domains...)
		return nil
	}

	projects := []database.ProjectRecord{
		{Domain: "docmost.local"},
		{Domain: ""},
		{Domain: "vikunja.localhost"},
	}

	if err := syncManagedDomains(projects); err != nil {
		t.Fatalf("sync managed domains: %v", err)
	}

	want := []string{"docmost.local", "vikunja.localhost"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected synced domains: got %v want %v", got, want)
	}
}

// Regresion: un proyecto registrado sin compose ni metadata de proxy no debe
// impedir que los demas queden publicados. Antes, 'devherd proxy apply'
// abortaba en el primero que fallara y ningun dominio se escribia.
func TestResolveExternalProjectsSkipsUnresolvableProjects(t *testing.T) {
	original := buildExternalProject
	t.Cleanup(func() {
		buildExternalProject = original
	})

	buildExternalProject = func(_ config.Config, project database.ProjectRecord) (proxy.ExternalProject, error) {
		if project.Name == "sin-metadata" {
			return proxy.ExternalProject{}, fmt.Errorf("project %q needs manifest proxy metadata or a supported framework for external proxy mode", project.Name)
		}

		return proxy.ExternalProject{Project: project, Domain: project.Name + ".localhost"}, nil
	}

	projects := []database.ProjectRecord{
		{Name: "sin-metadata"},
		{Name: "java-starter"},
	}

	resolved, skipped, err := resolveExternalProjects(config.Config{}, projects, false)
	if err != nil {
		t.Fatalf("resolve external projects: %v", err)
	}

	if len(resolved) != 1 || resolved[0].Domain != "java-starter.localhost" {
		t.Fatalf("el proyecto sano debio sobrevivir al que falla: %+v", resolved)
	}

	if !reflect.DeepEqual(skipped, []string{"sin-metadata"}) {
		t.Fatalf("unexpected skipped projects: %v", skipped)
	}
}

// Pedir un proyecto por nombre no omite nada: si ese falla, es error.
func TestResolveExternalProjectsFailsWhenProjectIsExplicit(t *testing.T) {
	original := buildExternalProject
	t.Cleanup(func() {
		buildExternalProject = original
	})

	buildExternalProject = func(_ config.Config, _ database.ProjectRecord) (proxy.ExternalProject, error) {
		return proxy.ExternalProject{}, fmt.Errorf("needs manifest proxy metadata")
	}

	projects := []database.ProjectRecord{{Name: "sin-metadata"}}

	if _, _, err := resolveExternalProjects(config.Config{}, projects, true); err == nil {
		t.Fatal("se esperaba error al pedir explicitamente un proyecto irresoluble")
	}
}
