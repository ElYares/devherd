package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devherd/devherd/internal/detector"
)

func TestCustomDomainSurvivesUpsert(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "devherd.db")

	manager := NewManager(dbPath)
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("ensure database: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	project := detector.Project{
		Name:      "hello-vue-flask-docker",
		Path:      "/tmp/hello-vue-flask-docker",
		Stack:     "node+python+docker",
		Framework: "vue+flask",
		Runtime:   "node+python",
	}

	if err := UpsertProject(ctx, db, project, "hello-vue-flask-docker.test"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := SetPrimaryDomain(ctx, db, project.Name, "mi-demo.test"); err != nil {
		t.Fatalf("set primary domain: %v", err)
	}

	if err := UpsertProject(ctx, db, project, "auto-generated.test"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	projects, err := ListProjects(ctx, db)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected one project, got %d", len(projects))
	}

	if projects[0].Domain != "mi-demo.test" {
		t.Fatalf("expected preserved custom domain, got %q", projects[0].Domain)
	}
}
