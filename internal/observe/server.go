package observe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
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

	return s.ListenAndServeOn(ctx, addr)
}

// ListenAndServeOn arranca el collector en varias direcciones a la vez. La
// primera es obligatoria; las demas son best effort, porque normalmente son el
// gateway de una red Docker que puede no existir todavia y eso no debe impedir
// que el collector arranque en loopback.
func (s Server) ListenAndServeOn(ctx context.Context, addrs ...string) error {
	addrs = uniqueAddrs(addrs)
	if len(addrs) == 0 {
		addrs = []string{DefaultAddr}
	}

	server := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	listeners := make([]net.Listener, 0, len(addrs))
	for i, addr := range addrs {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			if i == 0 {
				closeListeners(listeners)
				return fmt.Errorf("listen on %s: %w", addr, err)
			}

			slog.Warn("observe: extra collector address unavailable", "addr", addr, "err", err)
			continue
		}

		listeners = append(listeners, listener)
	}

	errc := make(chan error, len(listeners))
	for _, listener := range listeners {
		go func(listener net.Listener) {
			errc <- server.Serve(listener)
		}(listener)
	}

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

func uniqueAddrs(addrs []string) []string {
	seen := make(map[string]struct{}, len(addrs))
	unique := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}

		seen[addr] = struct{}{}
		unique = append(unique, addr)
	}

	return unique
}

func closeListeners(listeners []net.Listener) {
	for _, listener := range listeners {
		_ = listener.Close()
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

// logBackfillBatch acota lo que se intenta por tick. Cada objetivo cuesta una llamada
// a docker logs con timeout de 5 s, y el mismo goroutine tiene que volver a tiempo
// para la instantanea de contenedores del tick siguiente.
const logBackfillBatch = 25

func (s Server) pollObservedContainers(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		s.snapshotObservedContainers(ctx, "")
		s.backfillPendingLogs(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// backfillPendingLogs es la segunda pasada de captura. La ingesta pide
// --since t-30s --until t+30s en el instante t, asi que la mitad futura de la ventana
// llega vacia: esos logs todavia no existen. Aqui se vuelve por ellos cuando ya
// transcurrieron, mientras el contenedor sigue vivo y sus logs sin rotar.
func (s Server) backfillPendingLogs(ctx context.Context) {
	if s.docker == nil {
		return
	}

	targets, err := s.store.PendingLogBackfills(ctx, logBackfillBatch)
	if err != nil {
		slog.Warn("observe: list pending log backfills failed", "error", err)
		return
	}

	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}
		s.backfillEventLogs(ctx, target)
	}
}

func (s Server) backfillEventLogs(ctx context.Context, target LogBackfillTarget) {
	logs, err := s.docker.LogsBetween(ctx, target.Container, target.At, target.At.Add(logCaptureWindow), logCaptureLimit)
	if err != nil {
		// El contenedor pudo morir o cambiar de nombre. Marcarlo evita reintentar
		// cada 10 s un evento que ya no tiene de donde leer.
		slog.Warn("observe: backfill container logs failed",
			"event_id", target.EventID, "container", target.Container, "error", err)
		s.markLogBackfill(ctx, target.EventID, logsBackfillSkipped)
		return
	}

	for i := range logs {
		logs[i].EventID = target.EventID
		logs[i].Project = target.Project
		logs[i].Service = target.Service
		logs[i].Container = target.Container
	}

	if len(logs) > 0 {
		if err := s.store.StoreContainerLogs(ctx, target.EventID, logs); err != nil {
			// A diferencia del fallo de Docker, este es transitorio: el evento se
			// queda pendiente para reintentarlo en el tick siguiente.
			slog.Warn("observe: store backfilled container logs failed",
				"event_id", target.EventID, "log_count", len(logs), "error", err)
			return
		}
	}

	s.markLogBackfill(ctx, target.EventID, logsBackfillDone)
}

func (s Server) markLogBackfill(ctx context.Context, eventID string, state int) {
	if err := s.store.MarkLogBackfill(ctx, eventID, state); err != nil {
		slog.Warn("observe: mark log backfill failed", "event_id", eventID, "error", err)
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
