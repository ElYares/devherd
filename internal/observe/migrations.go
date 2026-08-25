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
	{
		table:  "events",
		column: "logs_backfilled",
		addDDL: `ALTER TABLE events ADD COLUMN logs_backfilled INTEGER NOT NULL DEFAULT 0`,
		// Los eventos que ya estaban en la base son de corridas anteriores del
		// collector: su ventana vencio hace rato y su contenedor probablemente ya
		// no existe. Marcarlos como vencidos evita que el primer barrido intente
		// una llamada a Docker por cada uno.
		backfill: `UPDATE events SET logs_backfilled = 2`,
	},
}

// indexAdditions corre despues de columnAdditions. Un indice parcial sobre una
// columna recien agregada no puede vivir en schema.sql, que se aplica antes: en una
// base que ya existia la columna todavia no esta cuando ese archivo corre.
var indexAdditions = []string{
	// Parcial a proposito: solo indexa los eventos pendientes de relleno, que son
	// unos pocos, en vez de la tabla entera de eventos que crece sin techo.
	`CREATE INDEX IF NOT EXISTS idx_observe_events_logs_pending ON events(id) WHERE logs_backfilled = 0`,
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

	for _, ddl := range indexAdditions {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("add observe index: %w", err)
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
