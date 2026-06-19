package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

type Manager struct {
	path string
}

func NewManager(path string) *Manager {
	return &Manager{path: path}
}

func (m *Manager) Ensure(ctx context.Context) (bool, error) {
	created := false

	if _, err := os.Stat(m.path); errors.Is(err, os.ErrNotExist) {
		created = true
	} else if err != nil {
		return false, fmt.Errorf("stat database: %w", err)
	}

	db, err := m.open()
	if err != nil {
		return false, err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		return false, err
	}

	return created, nil
}

// migrate aplica, en orden, las migraciones pendientes registrándolas en la
// tabla schema_migrations. Las migraciones deben ser idempotentes.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	for _, mig := range migrations {
		if applied[mig.Version] {
			continue
		}

		if _, err := db.ExecContext(ctx, mig.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", mig.Name, err)
		}

		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`,
			mig.Version, mig.Name,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", mig.Name, err)
		}
	}

	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func (m *Manager) Open() (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", m.path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}

func (m *Manager) open() (*sql.DB, error) {
	return m.Open()
}
