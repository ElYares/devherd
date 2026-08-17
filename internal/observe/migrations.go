package observe

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

// columnAddition describe una columna que se agrego cuando su tabla ya existia en
// bases reales. Observe aplica schema.sql con CREATE TABLE IF NOT EXISTS, que no
// toca una tabla ya creada, asi que sin esto una base anterior al cambio nunca
// veria la columna nueva. SQLite tampoco admite ADD COLUMN IF NOT EXISTS: de ahi
// la consulta previa a pragma_table_info, que es lo que hace idempotente al paso.
type columnAddition struct {
	table    string
	column   string
	addDDL   string
	backfill string
}

var columnAdditions = []columnAddition{
	{
		table:  "alerts",
		column: "cooldown_seconds",
		addDDL: `ALTER TABLE alerts ADD COLUMN cooldown_seconds INTEGER NOT NULL DEFAULT 0`,
		// Sin el relleno, las reglas que ya existian quedarian en 0, es decir sin
		// silencio: exactamente el ruido que el cooldown viene a resolver.
		backfill: `UPDATE alerts SET cooldown_seconds = CASE WHEN kind = 'error-rate' THEN window_seconds ELSE 900 END`,
	},
}

func applyColumnAdditions(ctx context.Context, db *sql.DB) error {
	for _, addition := range columnAdditions {
		exists, err := columnExists(ctx, db, addition.table, addition.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		if _, err := db.ExecContext(ctx, addition.addDDL); err != nil {
			return fmt.Errorf("add observe column %s.%s: %w", addition.table, addition.column, err)
		}
		if addition.backfill == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, addition.backfill); err != nil {
			return fmt.Errorf("backfill observe column %s.%s: %w", addition.table, addition.column, err)
		}
	}

	return nil
}

func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pragma_table_info(?)
		WHERE name = ?
	`, table, column).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect observe column %s.%s: %w", table, column, err)
	}

	return count > 0, nil
}
