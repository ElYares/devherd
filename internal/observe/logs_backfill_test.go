package observe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// countingDocker registra cada llamada de relleno. Contarlas es lo unico que
// distingue una segunda pasada de un bucle que le pregunta a Docker cada 10 s.
type countingDocker struct {
	mu      sync.Mutex
	calls   []logRange
	logs    []ContainerLog
	failure error
}

type logRange struct {
	container string
	since     time.Time
	until     time.Time
}

func (d *countingDocker) ObservedContainers(_ context.Context, _ string) ([]ObservedContainer, error) {
	return nil, nil
}

func (d *countingDocker) LogsAround(_ context.Context, _ string, _ time.Time, _ time.Duration, _ int) ([]ContainerLog, error) {
	return nil, nil
}

func (d *countingDocker) LogsBetween(_ context.Context, container string, since, until time.Time, _ int) ([]ContainerLog, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.calls = append(d.calls, logRange{container: container, since: since, until: until})
	if d.failure != nil {
		return nil, d.failure
	}

	return append([]ContainerLog(nil), d.logs...), nil
}

func (d *countingDocker) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.calls)
}

func newBackfillServer(t *testing.T, docker DockerRuntime) (Server, Store, *testClock) {
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
	store := NewStoreWithClock(db, clock.Now)

	return NewServerWithDocker(store, dbPath, docker), store, clock
}

func storeCorrelatedEvent(t *testing.T, store Store, message string) string {
	t.Helper()

	stored, err := store.StoreEvent(context.Background(), Event{
		Project:       "demo",
		Service:       "web",
		Container:     "demo_web_1",
		ExceptionType: "DemoError",
		Message:       message,
	})
	if err != nil {
		t.Fatalf("StoreEvent returned error: %v", err)
	}

	return stored.EventID
}

func backfillState(t *testing.T, store Store, eventID string) int {
	t.Helper()

	var state int
	if err := store.db.QueryRowContext(context.Background(), `
		SELECT logs_backfilled FROM events WHERE event_id = ?
	`, eventID).Scan(&state); err != nil {
		t.Fatalf("read logs_backfilled: %v", err)
	}

	return state
}

// Criterio 1: la ventana futura todavia no vencio, no hay nada nuevo que traer.
func TestBackfillLeavesEventsInsideWindowPending(t *testing.T) {
	docker := &countingDocker{}
	server, store, clock := newBackfillServer(t, docker)
	eventID := storeCorrelatedEvent(t, store, "failure inside window")

	clock.advance(10 * time.Second)
	server.backfillPendingLogs(context.Background())

	if docker.callCount() != 0 {
		t.Fatalf("expected no docker calls before the window closes, got %d", docker.callCount())
	}
	if state := backfillState(t, store, eventID); state != logsBackfillPending {
		t.Fatalf("expected event to stay pending, got state %d", state)
	}
}

// Criterios 2 y 3: una sola captura de la mitad futura, y no una por tick.
func TestBackfillCapturesFutureHalfExactlyOnce(t *testing.T) {
	docker := &countingDocker{logs: []ContainerLog{
		{Timestamp: "2026-08-17T10:00:03Z", Stream: "combined", Message: "stack trace line"},
	}}
	server, store, clock := newBackfillServer(t, docker)
	eventID := storeCorrelatedEvent(t, store, "failure with late trace")
	eventAt := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	clock.advance(40 * time.Second)
	for i := 0; i < 10; i++ {
		server.backfillPendingLogs(context.Background())
	}

	if docker.callCount() != 1 {
		t.Fatalf("expected exactly 1 docker call across 10 ticks, got %d", docker.callCount())
	}

	call := docker.calls[0]
	if call.container != "demo_web_1" {
		t.Fatalf("unexpected container in backfill call: %q", call.container)
	}
	if !call.since.Equal(eventAt) || !call.until.Equal(eventAt.Add(logCaptureWindow)) {
		t.Fatalf("expected range [%s, %s], got [%s, %s]",
			eventAt, eventAt.Add(logCaptureWindow), call.since, call.until)
	}

	if state := backfillState(t, store, eventID); state != logsBackfillDone {
		t.Fatalf("expected event marked as backfilled, got state %d", state)
	}

	timeline, err := store.Timeline(context.Background(), eventID)
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(timeline.Logs) != 1 || timeline.Logs[0].Message != "stack trace line" {
		t.Fatalf("expected the backfilled line in the timeline, got %#v", timeline.Logs)
	}
	if timeline.Logs[0].Container != "demo_web_1" || timeline.Logs[0].Project != "demo" {
		t.Fatalf("backfilled line lost its correlation: %#v", timeline.Logs[0])
	}
}

// Criterio 4: la segunda pasada solapa con la primera y no debe duplicar.
func TestBackfillDoesNotDuplicateOverlappingLines(t *testing.T) {
	ctx := context.Background()
	shared := ContainerLog{Timestamp: "2026-08-17T10:00:00.5Z", Stream: "combined", Message: "boom"}
	docker := &countingDocker{logs: []ContainerLog{
		shared,
		{Timestamp: "2026-08-17T10:00:04Z", Stream: "combined", Message: "after boom"},
	}}
	server, store, clock := newBackfillServer(t, docker)
	eventID := storeCorrelatedEvent(t, store, "failure with overlap")

	// Lo que ya dejo la primera pasada, en la ingesta.
	if err := store.StoreContainerLogs(ctx, eventID, []ContainerLog{shared}); err != nil {
		t.Fatalf("StoreContainerLogs returned error: %v", err)
	}

	clock.advance(40 * time.Second)
	server.backfillPendingLogs(ctx)

	timeline, err := store.Timeline(ctx, eventID)
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(timeline.Logs) != 2 {
		t.Fatalf("expected 2 lines after deduping the overlap, got %d: %#v", len(timeline.Logs), timeline.Logs)
	}

	seen := map[string]int{}
	for _, log := range timeline.Logs {
		seen[log.Timestamp+"|"+log.Stream+"|"+log.Message]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Fatalf("line %q stored %d times", key, count)
		}
	}
}

// Criterio 5: con dos pasadas, el orden de insercion deja de ser el cronologico.
func TestTimelineOrdersLogsChronologically(t *testing.T) {
	ctx := context.Background()
	docker := &countingDocker{}
	_, store, _ := newBackfillServer(t, docker)
	eventID := storeCorrelatedEvent(t, store, "failure with out of order logs")

	// Se insertan al reves a proposito: es lo que pasa cuando la segunda pasada
	// trae una linea con marca anterior a alguna de la primera.
	if err := store.StoreContainerLogs(ctx, eventID, []ContainerLog{
		{Timestamp: "2026-08-17T10:00:05Z", Stream: "combined", Message: "later"},
		{Timestamp: "2026-08-17T10:00:01Z", Stream: "combined", Message: "earlier"},
	}); err != nil {
		t.Fatalf("StoreContainerLogs returned error: %v", err)
	}

	timeline, err := store.Timeline(ctx, eventID)
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(timeline.Logs) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(timeline.Logs))
	}
	if timeline.Logs[0].Message != "earlier" || timeline.Logs[1].Message != "later" {
		t.Fatalf("expected chronological order, got %q then %q",
			timeline.Logs[0].Message, timeline.Logs[1].Message)
	}
}

// Criterio 6: el contenedor ya no existe. Un warning y no se reintenta.
func TestBackfillStopsRetryingUnreachableContainer(t *testing.T) {
	docker := &countingDocker{failure: errors.New("No such container: demo_web_1")}
	server, store, clock := newBackfillServer(t, docker)
	eventID := storeCorrelatedEvent(t, store, "failure on a dead container")

	clock.advance(40 * time.Second)
	for i := 0; i < 10; i++ {
		server.backfillPendingLogs(context.Background())
	}

	if docker.callCount() != 1 {
		t.Fatalf("expected 1 attempt on an unreachable container, got %d", docker.callCount())
	}
	if state := backfillState(t, store, eventID); state != logsBackfillSkipped {
		t.Fatalf("expected event marked as skipped, got state %d", state)
	}
}

// Criterio 7: el collector estuvo apagado. Ni una llamada a Docker al volver.
func TestBackfillExpiresEventsBeyondMaxAge(t *testing.T) {
	ctx := context.Background()
	docker := &countingDocker{}
	server, store, clock := newBackfillServer(t, docker)

	const count = 500
	for i := 0; i < count; i++ {
		storeCorrelatedEvent(t, store, fmt.Sprintf("stale failure number %s", numberWord(i)))
	}

	clock.advance(logBackfillMaxAge + time.Minute)
	server.backfillPendingLogs(ctx)

	if docker.callCount() != 0 {
		t.Fatalf("expected no docker calls for expired events, got %d", docker.callCount())
	}

	var pending int
	if err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events WHERE logs_backfilled = ?
	`, logsBackfillPending).Scan(&pending); err != nil {
		t.Fatalf("count pending events: %v", err)
	}
	if pending != count-logBackfillBatch {
		t.Fatalf("expected the sweep to retire one batch per tick, %d pending, got %d",
			count-logBackfillBatch, pending)
	}

	// El barrido se limpia solo: repetido hasta agotarse deja la cola vacia y
	// nunca llama a Docker.
	for pending > 0 {
		server.backfillPendingLogs(ctx)
		if err := store.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM events WHERE logs_backfilled = ?
		`, logsBackfillPending).Scan(&pending); err != nil {
			t.Fatalf("count pending events: %v", err)
		}
	}
	if docker.callCount() != 0 {
		t.Fatalf("expected no docker calls while draining expired events, got %d", docker.callCount())
	}
}

// Un evento que llego sin contenedor resuelto no tiene a quien preguntarle.
func TestBackfillSkipsEventsWithoutContainer(t *testing.T) {
	docker := &countingDocker{}
	server, store, clock := newBackfillServer(t, docker)

	stored, err := store.StoreEvent(context.Background(), Event{
		Project: "demo",
		Message: "failure with no container",
	})
	if err != nil {
		t.Fatalf("StoreEvent returned error: %v", err)
	}

	clock.advance(40 * time.Second)
	server.backfillPendingLogs(context.Background())

	if docker.callCount() != 0 {
		t.Fatalf("expected no docker calls without a container, got %d", docker.callCount())
	}
	if state := backfillState(t, store, stored.EventID); state != logsBackfillSkipped {
		t.Fatalf("expected event marked as skipped, got state %d", state)
	}
}

// Criterio 8: una base creada por un binario anterior a este cambio.
func TestMigrationAddsBackfillColumnAndRetiresOldEvents(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", dbPath))
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// El esquema tal como era antes de esta historia.
	legacySchema := strings.Replace(schemaSQL, "    logs_backfilled INTEGER NOT NULL DEFAULT 0,\n", "", 1)
	if legacySchema == schemaSQL {
		t.Fatal("legacy schema is identical to the current one; the column line moved")
	}
	if _, err := db.ExecContext(ctx, legacySchema); err != nil {
		t.Fatalf("apply legacy schema: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO issues (project, fingerprint, title, first_seen, last_seen, event_count)
		VALUES ('demo', 'abc', 'old failure', '2026-08-01T10:00:00Z', '2026-08-01T10:00:00Z', 1)
	`); err != nil {
		t.Fatalf("seed legacy issue: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO events (event_id, project, issue_id, timestamp, message)
		VALUES ('old-event', 'demo', 1, '2026-08-01T10:00:00Z', 'old failure')
	`); err != nil {
		t.Fatalf("seed legacy event: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO container_logs (event_id, project, message)
		VALUES ('old-event', 'demo', 'old line')
	`); err != nil {
		t.Fatalf("seed legacy log: %v", err)
	}

	if err := applyColumnAdditions(ctx, db); err != nil {
		t.Fatalf("applyColumnAdditions returned error: %v", err)
	}

	exists, err := columnExists(ctx, db, "events", "logs_backfilled")
	if err != nil {
		t.Fatalf("columnExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected logs_backfilled to be added to an existing events table")
	}

	var state int
	if err := db.QueryRowContext(ctx, `
		SELECT logs_backfilled FROM events WHERE event_id = 'old-event'
	`).Scan(&state); err != nil {
		t.Fatalf("read migrated event: %v", err)
	}
	if state != logsBackfillSkipped {
		t.Fatalf("expected pre-existing events retired as expired, got state %d", state)
	}

	var events, logs int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM container_logs`).Scan(&logs); err != nil {
		t.Fatalf("count container logs: %v", err)
	}
	if events != 1 || logs != 1 {
		t.Fatalf("migration lost rows: %d events, %d container logs", events, logs)
	}

	// Idempotente: abrir otra vez no vuelve a marcar ni falla por el indice.
	if err := applyColumnAdditions(ctx, db); err != nil {
		t.Fatalf("second applyColumnAdditions returned error: %v", err)
	}
}

// numberWord evita que el fingerprint agrupe los eventos: enmascara los numeros, asi
// que "failure 1" y "failure 2" serian el mismo issue.
func numberWord(n int) string {
	words := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	if n == 0 {
		return words[0]
	}

	var parts []string
	for n > 0 {
		parts = append([]string{words[n%10]}, parts...)
		n /= 10
	}

	return strings.Join(parts, "-")
}
