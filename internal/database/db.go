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

	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return false, fmt.Errorf("apply schema: %w", err)
	}

	return created, nil
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
