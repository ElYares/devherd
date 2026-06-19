package database

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	Version int
	Name    string
	SQL     string
}

// loadMigrations lee y ordena las migraciones embebidas por su número de versión
// (prefijo numérico del nombre, p. ej. 0001_init.sql -> 1).
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, err := versionFromName(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("migration %q: %w", entry.Name(), err)
		}

		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration{Version: version, Name: entry.Name(), SQL: string(data)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	return migrations, nil
}

func versionFromName(name string) (int, error) {
	prefix := name
	if idx := strings.IndexByte(name, '_'); idx >= 0 {
		prefix = name[:idx]
	}

	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("nombre sin prefijo numérico de versión")
	}

	return version, nil
}
