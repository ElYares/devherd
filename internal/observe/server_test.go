package observe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerAcceptsSimpleEvent(t *testing.T) {
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
	handler := NewServerWithDocker(store, dbPath, nil).Handler()

	request := httptest.NewRequest(http.MethodPost, "/api/demo/event", strings.NewReader(`{"message":"demo failure","exception_type":"DemoError","service":"web"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", response.Code)
	}

	var payload struct {
		EventID     string `json:"event_id"`
		Fingerprint string `json:"fingerprint"`
		IssueID     int64  `json:"issue_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.EventID == "" || payload.Fingerprint == "" || payload.IssueID == 0 {
		t.Fatalf("unexpected response payload: %#v", payload)
	}

	issues, err := store.ListIssues(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListIssues returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %d", len(issues))
	}
}

func TestEventPayloadFromEnvelope(t *testing.T) {
	payload, err := eventPayloadFromEnvelope([]byte("{}\n{\"type\":\"event\"}\n{\"message\":\"from envelope\"}\n"))
	if err != nil {
		t.Fatalf("eventPayloadFromEnvelope returned error: %v", err)
	}

	if string(payload) != `{"message":"from envelope"}` {
		t.Fatalf("unexpected envelope payload: %s", payload)
	}
}

func TestServerCorrelatesEventWithContainerLogs(t *testing.T) {
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
	handler := NewServerWithDocker(store, dbPath, fakeDockerRuntime{}).Handler()

	request := httptest.NewRequest(http.MethodPost, "/api/demo/event", strings.NewReader(`{"message":"demo failure","exception_type":"DemoError","service":"web","timestamp":"2026-05-22T10:00:00Z"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", response.Code)
	}

	var payload struct {
		EventID   string `json:"event_id"`
		Container string `json:"container"`
		LogCount  int    `json:"log_count"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Container != "demo_web_1" || payload.LogCount != 1 {
		t.Fatalf("unexpected response payload: %#v", payload)
	}

	timeline, err := store.Timeline(ctx, payload.EventID)
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(timeline.Logs) != 1 || timeline.Logs[0].Message != "log near failure" {
		t.Fatalf("unexpected timeline logs: %#v", timeline.Logs)
	}
	if timeline.Event.Container != "demo_web_1" {
		t.Fatalf("expected event container to be enriched, got %#v", timeline.Event)
	}
}

func TestServerServesObservePanelAPI(t *testing.T) {
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
	event, err := NormalizeEvent("demo", []byte(`{"message":"demo failure","exception_type":"DemoError","service":"web"}`))
	if err != nil {
		t.Fatalf("NormalizeEvent returned error: %v", err)
	}
	if _, err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("StoreEvent returned error: %v", err)
	}

	handler := NewServerWithDocker(store, dbPath, nil).Handler()

	panelRequest := httptest.NewRequest(http.MethodGet, "/observe", nil)
	panelResponse := httptest.NewRecorder()
	handler.ServeHTTP(panelResponse, panelRequest)
	if panelResponse.Code != http.StatusOK {
		t.Fatalf("expected panel status 200, got %d", panelResponse.Code)
	}
	if !strings.Contains(panelResponse.Body.String(), "DevHerd Observe") {
		t.Fatalf("expected observe panel HTML, got %q", panelResponse.Body.String())
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/api/observe/events?project=demo", nil)
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusOK {
		t.Fatalf("expected api status 200, got %d", apiResponse.Code)
	}

	var events []map[string]any
	if err := json.NewDecoder(apiResponse.Body).Decode(&events); err != nil {
		t.Fatalf("decode events response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("unexpected api events payload: %#v", events)
	}
	eventID, _ := events[0]["event_id"].(string)
	message, _ := events[0]["message"].(string)
	if eventID == "" || message != "demo failure" {
		t.Fatalf("unexpected api events payload: %#v", events)
	}
}

type fakeDockerRuntime struct{}

func (fakeDockerRuntime) ObservedContainers(ctx context.Context, project string) ([]ObservedContainer, error) {
	return []ObservedContainer{{
		ContainerID:  "abc123",
		Name:         "demo_web_1",
		Project:      "demo",
		Service:      "web",
		Image:        "nginx",
		Status:       "running",
		RestartCount: 0,
		Labels:       map[string]string{"devherd.project": "demo", "devherd.service": "web"},
	}}, nil
}

func (fakeDockerRuntime) LogsAround(ctx context.Context, container string, at time.Time, window time.Duration, limit int) ([]ContainerLog, error) {
	return []ContainerLog{{Timestamp: "2026-05-22T10:00:00Z", Message: "log near failure"}}, nil
}
