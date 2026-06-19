package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLoadMigrationsAreOrderedAndNumbered(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one migration")
	}
	if migrations[0].Version != 1 {
		t.Errorf("first migration version = %d, want 1", migrations[0].Version)
	}
	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			t.Errorf("migrations not strictly increasing at %d: %d <= %d", i, migrations[i].Version, migrations[i-1].Version)
		}
	}
}

func TestEnsureRecordsMigrationsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(filepath.Join(t.TempDir(), "devherd.db"))

	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	// Segunda llamada no debe fallar ni re-aplicar (idempotente).
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}

	migrations, _ := loadMigrations()
	if count != len(migrations) {
		t.Errorf("recorded migrations = %d, want %d", count, len(migrations))
	}

	// La tabla de dominio debe existir tras migrar.
	var name string
	if err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='projects'`).Scan(&name); err != nil {
		t.Fatalf("projects table missing after migrate: %v", err)
	}
}

// TestMigrateOnLegacyDatabaseRecordsBaseline simula una DB preexistente (tablas
// ya creadas sin schema_migrations, como las de usuarios actuales): migrate debe
// crear el registro, no fallar y dejar la baseline marcada como aplicada.
func TestMigrateOnLegacyDatabaseRecordsBaseline(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(filepath.Join(t.TempDir(), "legacy.db"))

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Aplica la baseline "a la antigua" (sin tabla de versiones).
	migrations, _ := loadMigrations()
	if _, err := db.ExecContext(ctx, migrations[0].SQL); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	// migrate sobre la DB legacy no debe fallar y debe registrar la baseline.
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("migrate on legacy db: %v", err)
	}

	var version int
	if err := db.QueryRowContext(ctx,
		`SELECT version FROM schema_migrations ORDER BY version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("baseline not recorded: %v", err)
	}
	if version != 1 {
		t.Errorf("baseline version = %d, want 1", version)
	}
}
