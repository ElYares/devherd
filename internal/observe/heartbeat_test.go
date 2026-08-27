package observe

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// clockedStore abre una base con un reloj que el test controla. Los huecos de
// cobertura solo se pueden probar moviendo el tiempo a voluntad.
func clockedStore(t *testing.T, now *time.Time) (Store, *sql.DB) {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "observe.db")
	manager := NewManager(dbPath)
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return NewStoreWithClock(db, func() time.Time { return *now }), db
}

// Una corrida del collector es una fila que se actualiza en su sitio, no una fila
// por latido: seis latidos por minuto darian 8.640 filas al dia para responder una
// pregunta que se contesta con dos fechas.
func TestHeartbeatUpdatesTheSessionInPlace(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	store, db := clockedStore(t, &now)
	ctx := context.Background()

	sessionID, err := store.StartCollectorSession(ctx)
	if err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}

	for i := 0; i < 5; i++ {
		now = now.Add(10 * time.Second)
		if err := store.Heartbeat(ctx, sessionID); err != nil {
			t.Fatalf("Heartbeat returned error: %v", err)
		}
	}

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM collector_sessions`).Scan(&rows); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 5 heartbeats to keep a single row, got %d", rows)
	}

	session, found, err := store.LastCollectorSession(ctx)
	if err != nil || !found {
		t.Fatalf("LastCollectorSession returned (%v, %v)", found, err)
	}
	if got := session.Duration(); got != 50*time.Second {
		t.Errorf("expected a 50s session, got %s", got)
	}
}

// El caso que da sentido a la tabla: un `observe issues` vacio dentro del hueco no
// significa que la aplicacion estuviera sana, sino que nadie estaba recibiendo.
func TestCoverageGapsFindsTheWindowWithNoCollector(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	// Primera corrida: 08:00 a 10:00.
	first, err := store.StartCollectorSession(ctx)
	if err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}
	now = now.Add(2 * time.Hour)
	if err := store.Heartbeat(ctx, first); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	// Cuatro horas muerto, y vuelve a las 14:00.
	now = now.Add(4 * time.Hour)
	second, err := store.StartCollectorSession(ctx)
	if err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}
	now = now.Add(time.Hour)
	if err := store.Heartbeat(ctx, second); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	gaps, err := store.CoverageGaps(ctx, time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC), time.Minute)
	if err != nil {
		t.Fatalf("CoverageGaps returned error: %v", err)
	}

	if len(gaps) != 1 {
		t.Fatalf("expected exactly one gap, got %d: %#v", len(gaps), gaps)
	}
	if got := gaps[0].Duration(); got != 4*time.Hour {
		t.Errorf("expected a 4h gap, got %s", got)
	}
	if !gaps[0].From.Equal(time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("expected the gap to start at 10:00, got %s", gaps[0].From)
	}
}

// Reiniciar el collector deja un hueco de segundos que no le interesa a nadie.
// Avisar de esos convierte el aviso en ruido, y un aviso que se ignora no avisa.
func TestCoverageGapsIgnoresGapsShorterThanTheThreshold(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	first, _ := store.StartCollectorSession(ctx)
	now = now.Add(time.Hour)
	if err := store.Heartbeat(ctx, first); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	// Reinicio de 5 segundos.
	now = now.Add(5 * time.Second)
	if _, err := store.StartCollectorSession(ctx); err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}

	gaps, err := store.CoverageGaps(ctx, time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC), time.Minute)
	if err != nil {
		t.Fatalf("CoverageGaps returned error: %v", err)
	}
	if len(gaps) != 0 {
		t.Errorf("a 5s restart should not be reported as a gap, got %#v", gaps)
	}
}

// El hueco inicial es el mas facil de olvidar y el mas grande: alguien que
// enciende el collector despues de una semana sin usarlo.
func TestCoverageGapsReportsTheGapBeforeTheFirstSession(t *testing.T) {
	now := time.Date(2026, 8, 26, 14, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	if _, err := store.StartCollectorSession(ctx); err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}

	since := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	gaps, err := store.CoverageGaps(ctx, since, time.Minute)
	if err != nil {
		t.Fatalf("CoverageGaps returned error: %v", err)
	}

	if len(gaps) != 1 {
		t.Fatalf("expected the leading gap to be reported, got %#v", gaps)
	}
	if got := gaps[0].Duration(); got != 6*time.Hour {
		t.Errorf("expected a 6h leading gap, got %s", got)
	}
}

// Dos collectores a la vez no dejan hueco, aunque sus corridas se ordenen raro.
func TestCoverageGapsIgnoresOverlappingSessions(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	first, _ := store.StartCollectorSession(ctx)
	now = now.Add(time.Minute)
	second, _ := store.StartCollectorSession(ctx)

	now = now.Add(2 * time.Hour)
	if err := store.Heartbeat(ctx, first); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if err := store.Heartbeat(ctx, second); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	gaps, err := store.CoverageGaps(ctx, time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC), time.Minute)
	if err != nil {
		t.Fatalf("CoverageGaps returned error: %v", err)
	}
	if len(gaps) != 0 {
		t.Errorf("overlapping sessions leave no gap, got %#v", gaps)
	}
}

// Una base sin ninguna corrida no tiene huecos que reportar: no se sabe nada, que
// no es lo mismo que saber que estuvo caido.
func TestCoverageGapsOnAnEmptyDatabase(t *testing.T) {
	now := time.Now().UTC()
	store, _ := clockedStore(t, &now)

	gaps, err := store.CoverageGaps(context.Background(), now.Add(-24*time.Hour), time.Minute)
	if err != nil {
		t.Fatalf("CoverageGaps returned error: %v", err)
	}
	if len(gaps) != 0 {
		t.Errorf("an empty database has nothing to report, got %#v", gaps)
	}

	if _, found, err := store.LastCollectorSession(context.Background()); err != nil || found {
		t.Errorf("expected no session, got (found=%v, err=%v)", found, err)
	}
}

// Un id invalido es un error del llamante, no algo que se traga en silencio.
func TestHeartbeatRejectsAnInvalidSession(t *testing.T) {
	now := time.Now().UTC()
	store, _ := clockedStore(t, &now)

	if err := store.Heartbeat(context.Background(), 0); err == nil {
		t.Error("expected an error for session id 0")
	}
}

// La trampa de la Decision 001: lo que funciona en una base nueva puede fallar en
// una que ya existe. `collector_sessions` es una tabla nueva, no una columna, asi
// que vive en schema.sql —que corre en cada Ensure— y no en columnAdditions. Esta
// prueba lo fija: sin ella, el fallo aparecería solo en la maquina de quien ya
// venia usando Observe, que es justo donde nadie lo prueba.
func TestCollectorSessionsAppearsOnAnAlreadyExistingDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "observe.db")
	manager := NewManager(dbPath)

	// Primera vida de la base, como si fuera de antes de esta funcionalidad.
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("first Ensure returned error: %v", err)
	}
	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE collector_sessions`); err != nil {
		t.Fatalf("simulate an older database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Segunda apertura: la tabla tiene que volver, y ser usable.
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("second Ensure returned error: %v", err)
	}
	db, err = manager.Open()
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	sessionID, err := store.StartCollectorSession(ctx)
	if err != nil {
		t.Fatalf("StartCollectorSession on the migrated database: %v", err)
	}
	if err := store.Heartbeat(ctx, sessionID); err != nil {
		t.Fatalf("Heartbeat on the migrated database: %v", err)
	}
}
