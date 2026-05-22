package observe

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreEventGroupsIssues(t *testing.T) {
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
	defer db.Close()

	store := NewStore(db)
	payload := []byte(`{"message":"demo failure","exception_type":"DemoError","service":"web"}`)

	event, err := NormalizeEvent("demo", payload)
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}
	if _, err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("StoreEvent first insert returned error: %v", err)
	}

	event, err = NormalizeEvent("demo", payload)
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}
	if _, err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("StoreEvent second insert returned error: %v", err)
	}

	issues, err := store.ListIssues(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListIssues returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one grouped issue, got %d", len(issues))
	}
	if issues[0].EventCount != 2 {
		t.Fatalf("expected issue event count 2, got %d", issues[0].EventCount)
	}

	events, err := store.ListEvents(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two stored events, got %d", len(events))
	}
}

func TestStoreContainersRecordsStatusAndRestartEvents(t *testing.T) {
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
	defer db.Close()

	store := NewStore(db)
	firstEvents, err := store.StoreContainers(ctx, []ObservedContainer{{
		ContainerID:  "abc",
		Name:         "demo_web",
		Project:      "demo",
		Service:      "web",
		Image:        "nginx",
		Status:       "running",
		RestartCount: 0,
		Labels:       map[string]string{"devherd.project": "demo"},
	}})
	if err != nil {
		t.Fatalf("StoreContainers first returned error: %v", err)
	}
	if len(firstEvents) != 1 || firstEvents[0].Kind != "seen" {
		t.Fatalf("expected seen event, got %#v", firstEvents)
	}

	secondEvents, err := store.StoreContainers(ctx, []ObservedContainer{{
		ContainerID:  "abc",
		Name:         "demo_web",
		Project:      "demo",
		Service:      "web",
		Image:        "nginx",
		Status:       "exited",
		RestartCount: 1,
		Labels:       map[string]string{"devherd.project": "demo"},
	}})
	if err != nil {
		t.Fatalf("StoreContainers second returned error: %v", err)
	}
	if len(secondEvents) != 2 {
		t.Fatalf("expected status and restart events, got %#v", secondEvents)
	}

	containers, err := store.ListContainers(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListContainers returned error: %v", err)
	}
	if len(containers) != 1 || containers[0].Status != "exited" || containers[0].RestartCount != 1 {
		t.Fatalf("unexpected containers: %#v", containers)
	}
}

func TestStoreCreatesAlertDeliveries(t *testing.T) {
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
	defer db.Close()

	store := NewStore(db)
	if _, err := store.AddAlert(ctx, Alert{Project: "demo", Kind: "new-issue"}); err != nil {
		t.Fatalf("AddAlert new-issue returned error: %v", err)
	}
	if _, err := store.AddAlert(ctx, Alert{Project: "demo", Kind: "error-rate", Threshold: 2, WindowSeconds: 3600}); err != nil {
		t.Fatalf("AddAlert error-rate returned error: %v", err)
	}
	if _, err := store.AddAlert(ctx, Alert{Project: "demo", Kind: "container-exit"}); err != nil {
		t.Fatalf("AddAlert container-exit returned error: %v", err)
	}
	if _, err := store.AddAlert(ctx, Alert{Project: "demo", Kind: "container-restart"}); err != nil {
		t.Fatalf("AddAlert container-restart returned error: %v", err)
	}

	payload := []byte(`{"message":"demo failure","exception_type":"DemoError","service":"web"}`)
	event, err := NormalizeEvent("demo", payload)
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}
	if _, err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("StoreEvent first returned error: %v", err)
	}

	event, err = NormalizeEvent("demo", payload)
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}
	if _, err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("StoreEvent second returned error: %v", err)
	}

	if _, err := store.StoreContainers(ctx, []ObservedContainer{{
		ContainerID:  "abc",
		Name:         "demo_web",
		Project:      "demo",
		Service:      "web",
		Status:       "running",
		RestartCount: 0,
	}}); err != nil {
		t.Fatalf("StoreContainers first returned error: %v", err)
	}
	if _, err := store.StoreContainers(ctx, []ObservedContainer{{
		ContainerID:  "abc",
		Name:         "demo_web",
		Project:      "demo",
		Service:      "web",
		Status:       "exited",
		RestartCount: 1,
	}}); err != nil {
		t.Fatalf("StoreContainers second returned error: %v", err)
	}

	deliveries, err := store.ListAlertDeliveries(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListAlertDeliveries returned error: %v", err)
	}

	got := map[string]int{}
	for _, delivery := range deliveries {
		got[delivery.Kind]++
	}

	for _, kind := range []string{"new-issue", "error-rate", "container-exit", "container-restart"} {
		if got[kind] != 1 {
			t.Fatalf("expected one %s alert delivery, got deliveries %#v", kind, deliveries)
		}
	}
}
