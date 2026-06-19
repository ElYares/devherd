package observe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const DefaultAddr = "127.0.0.1:9777"

type Server struct {
	store      Store
	dbPath     string
	docker     DockerRuntime
	pollDocker bool
}

func NewServer(store Store, dbPath string) Server {
	return Server{
		store:      store,
		dbPath:     dbPath,
		docker:     DockerCLI{},
		pollDocker: true,
	}
}

func NewServerWithDocker(store Store, dbPath string, docker DockerRuntime) Server {
	return Server{
		store:      store,
		dbPath:     dbPath,
		docker:     docker,
		pollDocker: docker != nil,
	}
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/observe/", s.handlePanelAPI)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/", s.handlePanel)
	return mux
}

func (s Server) ListenAndServe(ctx context.Context, addr string) error {
	if addr == "" {
		addr = DefaultAddr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		errc <- server.ListenAndServe()
	}()

	// El poller corre en su propio contexto para poder cancelarlo y esperar su
	// drenado tanto si el ctx padre termina como si el server muere por su cuenta.
	pollCtx, cancelPoll := context.WithCancel(ctx)
	defer cancelPoll()

	var wg sync.WaitGroup
	if s.pollDocker && s.docker != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.pollObservedContainers(pollCtx)
		}()
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		cancelPoll()
		wg.Wait()
		return ctx.Err()
	case err := <-errc:
		cancelPoll()
		wg.Wait()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": s.dbPath,
	})
}

func (s Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	project, action, ok := parseAPIPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "unsupported observe endpoint")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read request body: "+err.Error())
		return
	}

	payload := body
	if action == "envelope" {
		payload, err = eventPayloadFromEnvelope(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	event, err := NormalizeEvent(project, payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var logs []ContainerLog
	if s.docker != nil {
		correlator := NewCorrelator(s.store, s.docker)
		logs, err = correlator.CorrelateEvent(r.Context(), &event)
		if err != nil {
			slog.Warn("observe: correlate event with container logs failed",
				"project", project, "container", event.Container, "error", err)
		}
	}

	stored, err := s.store.StoreEvent(r.Context(), event)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(logs) > 0 {
		if err := s.store.StoreContainerLogs(r.Context(), stored.EventID, logs); err != nil {
			slog.Warn("observe: store container logs failed",
				"event_id", stored.EventID, "log_count", len(logs), "error", err)
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id":    stored.EventID,
		"issue_id":    stored.IssueID,
		"fingerprint": stored.Fingerprint,
		"container":   event.Container,
		"service":     event.Service,
		"log_count":   len(logs),
	})
}

func (s Server) pollObservedContainers(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		s.snapshotObservedContainers(ctx, "")
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s Server) snapshotObservedContainers(ctx context.Context, project string) {
	if s.docker == nil {
		return
	}

	containers, err := s.docker.ObservedContainers(ctx, project)
	if err != nil {
		slog.Warn("observe: list observed containers failed", "project", project, "error", err)
		return
	}
	if len(containers) == 0 {
		return
	}

	if _, err := s.store.StoreContainers(ctx, containers); err != nil {
		slog.Warn("observe: store observed containers failed",
			"project", project, "count", len(containers), "error", err)
	}
}

func parseAPIPath(path string) (string, string, bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", "", false
	}

	switch parts[1] {
	case "event", "store":
		return parts[0], "event", true
	case "envelope":
		return parts[0], "envelope", true
	default:
		return "", "", false
	}
}

func eventPayloadFromEnvelope(payload []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), 2<<20)

	line := 0
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		line++
		if line == 1 {
			continue
		}

		var header map[string]any
		if err := json.Unmarshal([]byte(text), &header); err != nil {
			return nil, fmt.Errorf("parse envelope item header: %w", err)
		}

		if !scanner.Scan() {
			return nil, fmt.Errorf("envelope item payload missing")
		}
		itemPayload := bytes.TrimSpace(scanner.Bytes())

		itemType := mapString(header, "type")
		if itemType == "event" || itemType == "transaction" {
			return append([]byte{}, itemPayload...), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read envelope: %w", err)
	}

	return nil, fmt.Errorf("envelope did not contain an event item")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}
