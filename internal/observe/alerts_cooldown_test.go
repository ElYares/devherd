package observe

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// testClock es un reloj que solo avanza cuando la prueba lo dice. Sin el no hay
// forma de comprobar que un cooldown expira sin dormir el tiempo real.
type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func newCooldownStore(t *testing.T) (Store, *testClock) {
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

	clock := &testClock{now: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)}

	return NewStoreWithClock(db, clock.Now), clock
}

func storeErrorEvents(t *testing.T, store Store, count int, message string) {
	t.Helper()

	ctx := context.Background()
	for i := 0; i < count; i++ {
		payload := fmt.Appendf(nil, `{"message":%q,"exception_type":"DemoError","service":"web"}`, message)
		event, err := NormalizeEvent("demo", payload)
		if err != nil {
			t.Fatalf("NormalizeEvent returned error: %v", err)
		}
		if _, err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("StoreEvent %d returned error: %v", i, err)
		}
	}
}

// distinctFailure produce mensajes que de verdad abren issues distintos: el
// fingerprint normaliza los numeros a <n>, asi que "failure 1" y "failure 2"
// serian el mismo issue y la prueba pasaria por el motivo equivocado.
func distinctFailure(i int) string {
	return "failure " + string(rune('a'+i))
}

func countDeliveries(t *testing.T, store Store, kind string) int {
	t.Helper()

	deliveries, err := store.ListAlertDeliveries(context.Background(), "demo", 1000)
	if err != nil {
		t.Fatalf("ListAlertDeliveries returned error: %v", err)
	}

	count := 0
	for _, delivery := range deliveries {
		if delivery.Kind == kind {
			count++
		}
	}

	return count
}

// Antes del cooldown, error-rate evaluaba el umbral en cada evento que entraba:
// con umbral 3 y ventana 5m, 50 errores producian 48 entregas.
func TestCooldownCollapsesErrorRateBurst(t *testing.T) {
	ctx := context.Background()
	store, _ := newCooldownStore(t)

	if _, err := store.AddAlert(ctx, Alert{
		Project:         "demo",
		Kind:            "error-rate",
		Threshold:       3,
		WindowSeconds:   300,
		CooldownSeconds: 300,
	}); err != nil {
		t.Fatalf("AddAlert returned error: %v", err)
	}

	storeErrorEvents(t, store, 50, "demo failure")

	if got := countDeliveries(t, store, "error-rate"); got != 1 {
		t.Fatalf("expected 1 error-rate delivery for a burst of 50 events, got %d", got)
	}
}

func TestCooldownExpiresAndAllowsANewDelivery(t *testing.T) {
	ctx := context.Background()
	store, clock := newCooldownStore(t)

	if _, err := store.AddAlert(ctx, Alert{
		Project:         "demo",
		Kind:            "error-rate",
		Threshold:       3,
		WindowSeconds:   300,
		CooldownSeconds: 300,
	}); err != nil {
		t.Fatalf("AddAlert returned error: %v", err)
	}

	storeErrorEvents(t, store, 50, "demo failure")
	if got := countDeliveries(t, store, "error-rate"); got != 1 {
		t.Fatalf("expected 1 delivery before advancing the clock, got %d", got)
	}

	clock.advance(6 * time.Minute)
	storeErrorEvents(t, store, 3, "demo failure")

	if got := countDeliveries(t, store, "error-rate"); got != 2 {
		t.Fatalf("expected a second delivery once the cooldown expired, got %d", got)
	}
}

// Una racha de issues genuinamente nuevos -- un despliegue que rompe 20 cosas
// distintas -- producia 20 avisos: el corte por issue ya existente no la tapa.
func TestCooldownCollapsesNewIssueBurst(t *testing.T) {
	ctx := context.Background()
	store, _ := newCooldownStore(t)

	if _, err := store.AddAlert(ctx, Alert{
		Project:         "demo",
		Kind:            "new-issue",
		CooldownSeconds: 900,
	}); err != nil {
		t.Fatalf("AddAlert returned error: %v", err)
	}

	for i := 0; i < 20; i++ {
		storeErrorEvents(t, store, 1, distinctFailure(i))
	}

	if got := countDeliveries(t, store, "new-issue"); got != 1 {
		t.Fatalf("expected 1 new-issue delivery for 20 distinct issues, got %d", got)
	}
}

// El comportamiento anterior sigue siendo alcanzable a proposito.
func TestZeroCooldownDeliversEveryTime(t *testing.T) {
	ctx := context.Background()
	store, _ := newCooldownStore(t)

	if _, err := store.AddAlert(ctx, Alert{
		Project:         "demo",
		Kind:            "new-issue",
		CooldownSeconds: 0,
	}); err != nil {
		t.Fatalf("AddAlert returned error: %v", err)
	}

	for i := 0; i < 5; i++ {
		storeErrorEvents(t, store, 1, distinctFailure(i))
	}

	if got := countDeliveries(t, store, "new-issue"); got != 5 {
		t.Fatalf("expected 5 new-issue deliveries without cooldown, got %d", got)
	}
}

// Fija la medida del hallazgo O4: sin cooldown, una rafaga de 50 eventos con
// umbral 3 produce 48 entregas. Es el numero contra el que se compara el arreglo.
func TestWithoutCooldownErrorRateDeliversOncePerEvent(t *testing.T) {
	ctx := context.Background()
	store, _ := newCooldownStore(t)

	if _, err := store.AddAlert(ctx, Alert{
		Project:         "demo",
		Kind:            "error-rate",
		Threshold:       3,
		WindowSeconds:   300,
		CooldownSeconds: 0,
	}); err != nil {
		t.Fatalf("AddAlert returned error: %v", err)
	}

	storeErrorEvents(t, store, 50, "demo failure")

	if got := countDeliveries(t, store, "error-rate"); got != 48 {
		t.Fatalf("expected 48 deliveries without cooldown, got %d", got)
	}
}

func TestAddAlertAppliesDefaultCooldownByKind(t *testing.T) {
	ctx := context.Background()
	store, _ := newCooldownStore(t)

	for _, alert := range []Alert{
		{Project: "demo", Kind: "error-rate", Threshold: 3, WindowSeconds: 300, CooldownSeconds: CooldownUnset},
		{Project: "demo", Kind: "new-issue", CooldownSeconds: CooldownUnset},
		{Project: "demo", Kind: "container-exit", CooldownSeconds: CooldownUnset},
	} {
		if _, err := store.AddAlert(ctx, alert); err != nil {
			t.Fatalf("AddAlert %s returned error: %v", alert.Kind, err)
		}
	}

	alerts, err := store.ListAlerts(ctx, "demo")
	if err != nil {
		t.Fatalf("ListAlerts returned error: %v", err)
	}

	want := map[string]int{"error-rate": 300, "new-issue": 900, "container-exit": 900}
	for _, alert := range alerts {
		if alert.CooldownSeconds != want[alert.Kind] {
			t.Fatalf("expected %s cooldown %d, got %d", alert.Kind, want[alert.Kind], alert.CooldownSeconds)
		}
	}
	if len(alerts) != len(want) {
		t.Fatalf("expected %d alerts, got %d", len(want), len(alerts))
	}
}

// Observe no tiene migraciones versionadas: schema.sql se aplica con CREATE TABLE
// IF NOT EXISTS, que no agrega columnas a una tabla ya creada. Una base anterior a
// este cambio tiene que ganar la columna sin perder ni una fila.
func TestEnsureAddsCooldownToAnExistingDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "observe.db")

	legacy, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open legacy database returned error: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `
		CREATE TABLE alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL,
			threshold INTEGER NOT NULL DEFAULT 1,
			window_seconds INTEGER NOT NULL DEFAULT 300,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE alert_deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_id INTEGER NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL,
			subject TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO alerts (project, kind, threshold, window_seconds) VALUES ('demo', 'error-rate', 3, 600);
		INSERT INTO alerts (project, kind, threshold, window_seconds) VALUES ('demo', 'new-issue', 1, 300);
		INSERT INTO alert_deliveries (alert_id, project, kind, subject, message, created_at)
		VALUES (1, 'demo', 'error-rate', 'entrega vieja', 'de antes del cooldown', '2026-08-01 10:00:00');
	`); err != nil {
		t.Fatalf("seed legacy database returned error: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database returned error: %v", err)
	}

	manager := NewManager(dbPath)
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	// Idempotencia: abrir dos veces no puede fallar ni pisar lo migrado.
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("second Ensure returned error: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	alerts, err := store.ListAlerts(ctx, "demo")
	if err != nil {
		t.Fatalf("ListAlerts returned error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected the 2 existing alerts to survive, got %d", len(alerts))
	}

	// error-rate hereda su ventana; los demas kinds, el fijo de 15 minutos.
	want := map[string]int{"error-rate": 600, "new-issue": 900}
	for _, alert := range alerts {
		if alert.CooldownSeconds != want[alert.Kind] {
			t.Fatalf("expected %s cooldown %d after the migration, got %d", alert.Kind, want[alert.Kind], alert.CooldownSeconds)
		}
	}

	deliveries, err := store.ListAlertDeliveries(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListAlertDeliveries returned error: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].Subject != "entrega vieja" {
		t.Fatalf("expected the existing delivery to survive, got %#v", deliveries)
	}
}
