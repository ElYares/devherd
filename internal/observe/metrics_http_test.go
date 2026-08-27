package observe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func metricsHandler(t *testing.T) http.Handler {
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

	return NewServerWithDocker(NewStore(db), dbPath, nil).Handler()
}

// El header lleva la version del formato. No es adorno: un scraper que no la
// reconoce cae a texto plano y deja de interpretar los tipos.
func TestMetricsEndpointAnnouncesThePrometheusFormat(t *testing.T) {
	response := httptest.NewRecorder()
	metricsHandler(t).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != MetricsContentType {
		t.Errorf("expected %q, got %q", MetricsContentType, got)
	}
}

// Una base sin datos responde 200 con las metricas en cero, no un cuerpo vacio ni
// un 404: un scrape que falla y uno que devuelve cero son cosas distintas, y
// Prometheus las trata distinto.
func TestMetricsEndpointOnAnEmptyDatabaseIsStillAValidScrape(t *testing.T) {
	response := httptest.NewRecorder()
	metricsHandler(t).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 on an empty database, got %d", response.Code)
	}

	body := response.Body.String()
	if strings.TrimSpace(body) == "" {
		t.Fatal("an empty database must still produce a document")
	}
	if !strings.Contains(body, "# TYPE devherd_observe_issues gauge") {
		t.Errorf("the families should be declared even with no samples, got:\n%s", body)
	}
	if !strings.Contains(body, "devherd_observe_collector_gap_seconds ") {
		t.Errorf("the gap gauge should always have a sample, got:\n%s", body)
	}
}

// Es una ruta de lectura. Un POST no puede pasar por un scrape.
func TestMetricsEndpointRejectsNonGet(t *testing.T) {
	response := httptest.NewRecorder()
	metricsHandler(t).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/metrics", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", response.Code)
	}
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("a 405 must say what is allowed, got %q", got)
	}
}

// La ruta es nueva y no puede haber cambiado el comportamiento de las que ya
// estaban.
func TestMetricsEndpointDoesNotShadowTheExistingRoutes(t *testing.T) {
	handler := metricsHandler(t)

	for _, path := range []string{"/health", "/observe"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))

			if response.Code >= 500 {
				t.Errorf("%s broke with %d: %s", path, response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "devherd_observe_") {
				t.Errorf("%s is serving the metrics document", path)
			}
		})
	}
}

// Los datos que entran por la ingesta salen por el scrape. Es la prueba de que las
// dos mitades hablan de lo mismo.
func TestMetricsEndpointReflectsIngestedEvents(t *testing.T) {
	handler := metricsHandler(t)

	ingest := httptest.NewRecorder()
	handler.ServeHTTP(ingest, httptest.NewRequest(http.MethodPost, "/api/demo/event",
		strings.NewReader(`{"message":"a demo failure","exception_type":"DemoError","level":"error"}`)))
	if ingest.Code >= 400 {
		t.Fatalf("ingest failed with %d: %s", ingest.Code, ingest.Body.String())
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !strings.Contains(response.Body.String(), `devherd_observe_issues{level="error",project="demo"} 1`) {
		t.Errorf("the ingested event should show up in the scrape, got:\n%s", response.Body.String())
	}
}
