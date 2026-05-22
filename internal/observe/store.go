package observe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/config"
	_ "modernc.org/sqlite"
)

type Manager struct {
	path string
}

type Store struct {
	db *sql.DB
}

type StoredEvent struct {
	EventID     string `json:"event_id"`
	IssueID     int64  `json:"issue_id"`
	Fingerprint string `json:"fingerprint"`
}

type Issue struct {
	ID            int64  `json:"id"`
	Project       string `json:"project"`
	Fingerprint   string `json:"fingerprint"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Level         string `json:"level"`
	Platform      string `json:"platform"`
	Service       string `json:"service"`
	Container     string `json:"container"`
	ExceptionType string `json:"exception_type"`
	Culprit       string `json:"culprit"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
	EventCount    int    `json:"event_count"`
}

type EventRecord struct {
	ID            int64  `json:"id"`
	EventID       string `json:"event_id"`
	Project       string `json:"project"`
	IssueID       int64  `json:"issue_id"`
	Timestamp     string `json:"timestamp"`
	Level         string `json:"level"`
	Platform      string `json:"platform"`
	Service       string `json:"service"`
	Container     string `json:"container"`
	ExceptionType string `json:"exception_type"`
	Message       string `json:"message"`
	Culprit       string `json:"culprit"`
	Transaction   string `json:"transaction"`
	Environment   string `json:"environment"`
	Release       string `json:"release"`
}

type Timeline struct {
	Event           EventRecord      `json:"event"`
	Logs            []ContainerLog   `json:"logs"`
	ContainerEvents []ContainerEvent `json:"container_events"`
}

type Alert struct {
	ID            int64  `json:"id"`
	Project       string `json:"project"`
	Kind          string `json:"kind"`
	Threshold     int    `json:"threshold"`
	WindowSeconds int    `json:"window_seconds"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
}

type AlertDelivery struct {
	ID        int64  `json:"id"`
	AlertID   int64  `json:"alert_id"`
	Project   string `json:"project"`
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type CleanupResult struct {
	Events          int64
	ContainerLogs   int64
	ContainerEvents int64
	AlertDeliveries int64
	Issues          int64
}

func DefaultDBPath(paths config.Paths) string {
	return filepath.Join(paths.DataDir, "observability", "devherd-observe.db")
}

func NewManager(path string) *Manager {
	return &Manager{path: path}
}

func (m *Manager) Ensure(ctx context.Context) (bool, error) {
	created := false
	if _, err := os.Stat(m.path); errors.Is(err, os.ErrNotExist) {
		created = true
	} else if err != nil {
		return false, fmt.Errorf("stat observe database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return false, fmt.Errorf("create observe database directory: %w", err)
	}

	db, err := m.Open()
	if err != nil {
		return false, err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, fmt.Errorf("ping observe database: %w", err)
	}

	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return false, fmt.Errorf("apply observe schema: %w", err)
	}

	return created, nil
}

func (m *Manager) Open() (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", m.path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open observe database: %w", err)
	}

	return db, nil
}

func NewStore(db *sql.DB) Store {
	return Store{db: db}
}

func (s Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s Store) StoreEvent(ctx context.Context, event Event) (StoredEvent, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if event.Timestamp == "" {
		event.Timestamp = now
	}
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.Environment == "" {
		event.Environment = "local"
	}
	if event.Level == "" {
		event.Level = "error"
	}
	if event.Title == "" {
		event.Title = eventTitle(event)
	}
	if event.Fingerprint == "" {
		event.Fingerprint = Fingerprint(event)
	}
	if event.RawPayload == "" {
		event.RawPayload = "{}"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("begin observe transaction: %w", err)
	}
	defer tx.Rollback()

	issueWasNew, err := issueIsNew(ctx, tx, event.Project, event.Fingerprint)
	if err != nil {
		return StoredEvent{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO issues (
			project,
			fingerprint,
			title,
			status,
			level,
			platform,
			service,
			container,
			exception_type,
			culprit,
			first_seen,
			last_seen,
			event_count,
			updated_at
		)
		VALUES (?, ?, ?, 'new', ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(project, fingerprint) DO UPDATE SET
			title = excluded.title,
			level = excluded.level,
			platform = excluded.platform,
			service = excluded.service,
			container = excluded.container,
			exception_type = excluded.exception_type,
			culprit = excluded.culprit,
			last_seen = excluded.last_seen,
			event_count = issues.event_count + 1,
			status = CASE WHEN issues.status = 'resolved' THEN 'new' ELSE issues.status END,
			updated_at = excluded.updated_at
	`, event.Project, event.Fingerprint, event.Title, event.Level, event.Platform, event.Service, event.Container, event.ExceptionType, event.Culprit, event.Timestamp, event.Timestamp, now)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("upsert observe issue: %w", err)
	}

	var issueID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM issues
		WHERE project = ? AND fingerprint = ?
		LIMIT 1
	`, event.Project, event.Fingerprint).Scan(&issueID); err != nil {
		return StoredEvent{}, fmt.Errorf("lookup observe issue: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events (
			event_id,
			project,
			issue_id,
			timestamp,
			level,
			platform,
			service,
			container,
			exception_type,
			message,
			culprit,
			transaction_name,
			environment,
			release,
			raw_payload
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.EventID, event.Project, issueID, event.Timestamp, event.Level, event.Platform, event.Service, event.Container, event.ExceptionType, event.Message, event.Culprit, event.Transaction, event.Environment, event.Release, event.RawPayload)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("insert observe event: %w", err)
	}

	if err := insertEventAlertDeliveries(ctx, tx, event, issueID, issueWasNew, now); err != nil {
		return StoredEvent{}, err
	}

	if err := tx.Commit(); err != nil {
		return StoredEvent{}, fmt.Errorf("commit observe transaction: %w", err)
	}

	return StoredEvent{
		EventID:     event.EventID,
		IssueID:     issueID,
		Fingerprint: event.Fingerprint,
	}, nil
}

func (s Store) StoreContainers(ctx context.Context, containers []ObservedContainer) ([]ContainerEvent, error) {
	if len(containers) == 0 {
		return nil, nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin observe container transaction: %w", err)
	}
	defer tx.Rollback()

	var events []ContainerEvent
	for _, container := range containers {
		if container.ContainerID == "" {
			continue
		}

		labelsJSON, err := json.Marshal(container.Labels)
		if err != nil {
			return nil, fmt.Errorf("encode container labels: %w", err)
		}

		var previousStatus string
		var previousRestartCount int
		var exists int
		err = tx.QueryRowContext(ctx, `
			SELECT status, restart_count, 1
			FROM containers
			WHERE container_id = ?
			LIMIT 1
		`, container.ContainerID).Scan(&previousStatus, &previousRestartCount, &exists)
		if errors.Is(err, sql.ErrNoRows) {
			exists = 0
		} else if err != nil {
			return nil, fmt.Errorf("lookup observed container: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO containers (
				container_id,
				name,
				project,
				service,
				image,
				status,
				restart_count,
				labels_json,
				first_seen,
				last_seen,
				updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(container_id) DO UPDATE SET
				name = excluded.name,
				project = excluded.project,
				service = excluded.service,
				image = excluded.image,
				status = excluded.status,
				restart_count = excluded.restart_count,
				labels_json = excluded.labels_json,
				last_seen = excluded.last_seen,
				updated_at = excluded.updated_at
		`, container.ContainerID, container.Name, container.Project, container.Service, container.Image, container.Status, container.RestartCount, string(labelsJSON), now, now, now)
		if err != nil {
			return nil, fmt.Errorf("upsert observed container: %w", err)
		}

		events = append(events, containerEventsForSnapshot(container, exists == 1, previousStatus, previousRestartCount, now)...)
	}

	for _, event := range events {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO container_events (
				container_id,
				name,
				project,
				service,
				kind,
				status,
				restart_count,
				message,
				created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.ContainerID, event.Name, event.Project, event.Service, event.Kind, event.Status, event.RestartCount, event.Message, event.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert observed container event: %w", err)
		}
		if err := insertContainerAlertDeliveries(ctx, tx, event); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit observe container transaction: %w", err)
	}

	return events, nil
}

func (s Store) AddAlert(ctx context.Context, alert Alert) (int64, error) {
	alert.Kind = strings.TrimSpace(alert.Kind)
	if alert.Kind == "" {
		return 0, fmt.Errorf("alert kind is required")
	}
	if alert.Threshold <= 0 {
		alert.Threshold = 1
	}
	if alert.WindowSeconds <= 0 {
		alert.WindowSeconds = 300
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO alerts (project, kind, threshold, window_seconds, enabled)
		VALUES (?, ?, ?, ?, 1)
	`, strings.TrimSpace(alert.Project), alert.Kind, alert.Threshold, alert.WindowSeconds)
	if err != nil {
		return 0, fmt.Errorf("insert observe alert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read observe alert id: %w", err)
	}

	return id, nil
}

func (s Store) ListAlerts(ctx context.Context, project string) ([]Alert, error) {
	query := `
		SELECT id, project, kind, threshold, window_seconds, enabled, created_at
		FROM alerts
	`
	args := []any{}
	if project != "" {
		query += ` WHERE project = ? OR project = ''`
		args = append(args, project)
	}
	query += ` ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observe alerts: %w", err)
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var alert Alert
		var enabled int
		if err := rows.Scan(&alert.ID, &alert.Project, &alert.Kind, &alert.Threshold, &alert.WindowSeconds, &enabled, &alert.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan observe alert: %w", err)
		}
		alert.Enabled = enabled == 1
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe alerts: %w", err)
	}

	return alerts, nil
}

func (s Store) RemoveAlert(ctx context.Context, id int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM alerts WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("remove observe alert: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read removed observe alert count: %w", err)
	}
	return count > 0, nil
}

func (s Store) ListAlertDeliveries(ctx context.Context, project string, limit int) ([]AlertDelivery, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, alert_id, project, kind, subject, message, created_at
		FROM alert_deliveries
	`
	args := []any{}
	if project != "" {
		query += ` WHERE project = ?`
		args = append(args, project)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observe alert deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []AlertDelivery
	for rows.Next() {
		var delivery AlertDelivery
		if err := rows.Scan(&delivery.ID, &delivery.AlertID, &delivery.Project, &delivery.Kind, &delivery.Subject, &delivery.Message, &delivery.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan observe alert delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe alert deliveries: %w", err)
	}

	return deliveries, nil
}

func (s Store) StoreContainerLogs(ctx context.Context, eventID string, logs []ContainerLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observe logs transaction: %w", err)
	}
	defer tx.Rollback()

	for _, log := range logs {
		if log.EventID == "" {
			log.EventID = eventID
		}
		if log.Stream == "" {
			log.Stream = "combined"
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO container_logs (
				event_id,
				project,
				service,
				container,
				timestamp,
				stream,
				message
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, log.EventID, log.Project, log.Service, log.Container, log.Timestamp, log.Stream, log.Message)
		if err != nil {
			return fmt.Errorf("insert observe container log: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit observe logs transaction: %w", err)
	}

	return nil
}

func (s Store) ListIssues(ctx context.Context, project string, limit int) ([]Issue, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, project, fingerprint, title, status, level, platform, service, container,
			exception_type, culprit, first_seen, last_seen, event_count
		FROM issues
	`
	args := []any{}
	if project != "" {
		query += ` WHERE project = ?`
		args = append(args, project)
	}
	query += ` ORDER BY last_seen DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observe issues: %w", err)
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.Project, &issue.Fingerprint, &issue.Title, &issue.Status, &issue.Level, &issue.Platform, &issue.Service, &issue.Container, &issue.ExceptionType, &issue.Culprit, &issue.FirstSeen, &issue.LastSeen, &issue.EventCount); err != nil {
			return nil, fmt.Errorf("scan observe issue: %w", err)
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe issues: %w", err)
	}

	return issues, nil
}

func (s Store) ListEvents(ctx context.Context, project string, limit int) ([]EventRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, event_id, project, issue_id, timestamp, level, platform, service, container,
			exception_type, message, culprit, transaction_name, environment, release
		FROM events
	`
	args := []any{}
	if project != "" {
		query += ` WHERE project = ?`
		args = append(args, project)
	}
	query += ` ORDER BY timestamp DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observe events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var event EventRecord
		if err := rows.Scan(&event.ID, &event.EventID, &event.Project, &event.IssueID, &event.Timestamp, &event.Level, &event.Platform, &event.Service, &event.Container, &event.ExceptionType, &event.Message, &event.Culprit, &event.Transaction, &event.Environment, &event.Release); err != nil {
			return nil, fmt.Errorf("scan observe event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe events: %w", err)
	}

	return events, nil
}

func (s Store) ListContainers(ctx context.Context, project string, limit int) ([]ObservedContainer, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT container_id, name, project, service, image, status, restart_count, labels_json
		FROM containers
	`
	args := []any{}
	if project != "" {
		query += ` WHERE project = ?`
		args = append(args, project)
	}
	query += ` ORDER BY last_seen DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observed containers: %w", err)
	}
	defer rows.Close()

	var containers []ObservedContainer
	for rows.Next() {
		var container ObservedContainer
		var labelsJSON string
		if err := rows.Scan(&container.ContainerID, &container.Name, &container.Project, &container.Service, &container.Image, &container.Status, &container.RestartCount, &labelsJSON); err != nil {
			return nil, fmt.Errorf("scan observed container: %w", err)
		}
		_ = json.Unmarshal([]byte(labelsJSON), &container.Labels)
		containers = append(containers, container)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observed containers: %w", err)
	}

	return containers, nil
}

func (s Store) Timeline(ctx context.Context, eventID string) (Timeline, error) {
	var timeline Timeline

	err := s.db.QueryRowContext(ctx, `
		SELECT id, event_id, project, issue_id, timestamp, level, platform, service, container,
			exception_type, message, culprit, transaction_name, environment, release
		FROM events
		WHERE event_id = ?
		LIMIT 1
	`, eventID).Scan(&timeline.Event.ID, &timeline.Event.EventID, &timeline.Event.Project, &timeline.Event.IssueID, &timeline.Event.Timestamp, &timeline.Event.Level, &timeline.Event.Platform, &timeline.Event.Service, &timeline.Event.Container, &timeline.Event.ExceptionType, &timeline.Event.Message, &timeline.Event.Culprit, &timeline.Event.Transaction, &timeline.Event.Environment, &timeline.Event.Release)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Timeline{}, fmt.Errorf("observe event %q not found", eventID)
		}
		return Timeline{}, fmt.Errorf("lookup observe event: %w", err)
	}

	logRows, err := s.db.QueryContext(ctx, `
		SELECT event_id, project, service, container, timestamp, stream, message
		FROM container_logs
		WHERE event_id = ?
		ORDER BY id ASC
	`, eventID)
	if err != nil {
		return Timeline{}, fmt.Errorf("list observe timeline logs: %w", err)
	}
	defer logRows.Close()
	for logRows.Next() {
		var log ContainerLog
		if err := logRows.Scan(&log.EventID, &log.Project, &log.Service, &log.Container, &log.Timestamp, &log.Stream, &log.Message); err != nil {
			return Timeline{}, fmt.Errorf("scan observe timeline log: %w", err)
		}
		timeline.Logs = append(timeline.Logs, log)
	}
	if err := logRows.Err(); err != nil {
		return Timeline{}, fmt.Errorf("iterate observe timeline logs: %w", err)
	}

	eventRows, err := s.db.QueryContext(ctx, `
		SELECT id, container_id, name, project, service, kind, status, restart_count, message, created_at
		FROM container_events
		WHERE project = ?
		  AND (? = '' OR service = ? OR name = ?)
		ORDER BY created_at DESC
		LIMIT 20
	`, timeline.Event.Project, timeline.Event.Service, timeline.Event.Service, timeline.Event.Container)
	if err != nil {
		return Timeline{}, fmt.Errorf("list observe container events: %w", err)
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var event ContainerEvent
		if err := eventRows.Scan(&event.ID, &event.ContainerID, &event.Name, &event.Project, &event.Service, &event.Kind, &event.Status, &event.RestartCount, &event.Message, &event.CreatedAt); err != nil {
			return Timeline{}, fmt.Errorf("scan observe container event: %w", err)
		}
		timeline.ContainerEvents = append(timeline.ContainerEvents, event)
	}
	if err := eventRows.Err(); err != nil {
		return Timeline{}, fmt.Errorf("iterate observe container events: %w", err)
	}

	return timeline, nil
}

func (s Store) Cleanup(ctx context.Context, days int) (CleanupResult, error) {
	if days <= 0 {
		return CleanupResult{}, fmt.Errorf("days must be greater than zero")
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("begin observe cleanup transaction: %w", err)
	}
	defer tx.Rollback()

	var result CleanupResult
	var execErr error
	result.ContainerLogs, execErr = execDelete(ctx, tx, `DELETE FROM container_logs WHERE datetime(created_at) < datetime(?)`, cutoff)
	if execErr != nil {
		return CleanupResult{}, execErr
	}
	result.ContainerEvents, execErr = execDelete(ctx, tx, `DELETE FROM container_events WHERE datetime(created_at) < datetime(?)`, cutoff)
	if execErr != nil {
		return CleanupResult{}, execErr
	}
	result.AlertDeliveries, execErr = execDelete(ctx, tx, `DELETE FROM alert_deliveries WHERE datetime(created_at) < datetime(?)`, cutoff)
	if execErr != nil {
		return CleanupResult{}, execErr
	}
	result.Events, execErr = execDelete(ctx, tx, `DELETE FROM events WHERE datetime(timestamp) < datetime(?)`, cutoff)
	if execErr != nil {
		return CleanupResult{}, execErr
	}
	result.Issues, execErr = execDelete(ctx, tx, `DELETE FROM issues WHERE datetime(last_seen) < datetime(?)`, cutoff)
	if execErr != nil {
		return CleanupResult{}, execErr
	}

	if err := tx.Commit(); err != nil {
		return CleanupResult{}, fmt.Errorf("commit observe cleanup transaction: %w", err)
	}

	return result, nil
}

func containerEventsForSnapshot(container ObservedContainer, exists bool, previousStatus string, previousRestartCount int, timestamp string) []ContainerEvent {
	base := ContainerEvent{
		ContainerID:  container.ContainerID,
		Name:         container.Name,
		Project:      container.Project,
		Service:      container.Service,
		Status:       container.Status,
		RestartCount: container.RestartCount,
		CreatedAt:    timestamp,
	}

	if !exists {
		base.Kind = "seen"
		base.Message = fmt.Sprintf("container %s observed with status %s", container.Name, container.Status)
		return []ContainerEvent{base}
	}

	var events []ContainerEvent
	if previousStatus != "" && previousStatus != container.Status {
		event := base
		event.Kind = "status"
		event.Message = fmt.Sprintf("container %s changed status from %s to %s", container.Name, previousStatus, container.Status)
		events = append(events, event)
	}

	if container.RestartCount > previousRestartCount {
		event := base
		event.Kind = "restart"
		event.Message = fmt.Sprintf("container %s restart count changed from %d to %d", container.Name, previousRestartCount, container.RestartCount)
		events = append(events, event)
	}

	return events
}

func issueIsNew(ctx context.Context, tx *sql.Tx, project, fingerprint string) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM issues
		WHERE project = ? AND fingerprint = ?
		LIMIT 1
	`, project, fingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("lookup observe issue before upsert: %w", err)
	}

	return false, nil
}

func insertEventAlertDeliveries(ctx context.Context, tx *sql.Tx, event Event, issueID int64, issueWasNew bool, now string) error {
	alerts, err := matchingAlerts(ctx, tx, event.Project)
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		switch alert.Kind {
		case "new-issue":
			if !issueWasNew {
				continue
			}
			if err := insertAlertDelivery(ctx, tx, alert, event.Project, "new issue", event.Title, now); err != nil {
				return err
			}
		case "error-rate":
			windowStart := time.Now().UTC().Add(-time.Duration(alert.WindowSeconds) * time.Second).Format(time.RFC3339Nano)
			var count int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM events
				WHERE project = ?
				  AND timestamp >= ?
			`, event.Project, windowStart).Scan(&count); err != nil {
				return fmt.Errorf("count observe error-rate window: %w", err)
			}
			if count >= alert.Threshold {
				subject := fmt.Sprintf("error rate threshold reached: %d events", count)
				message := fmt.Sprintf("%s reached %d events in %d seconds near issue %d", event.Project, count, alert.WindowSeconds, issueID)
				if err := insertAlertDelivery(ctx, tx, alert, event.Project, subject, message, now); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func insertContainerAlertDeliveries(ctx context.Context, tx *sql.Tx, event ContainerEvent) error {
	alerts, err := matchingAlerts(ctx, tx, event.Project)
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		switch alert.Kind {
		case "container-exit":
			if event.Kind != "status" || event.Status != "exited" {
				continue
			}
		case "container-restart":
			if event.Kind != "restart" {
				continue
			}
		default:
			continue
		}
		if err := insertAlertDelivery(ctx, tx, alert, event.Project, event.Kind+" "+event.Name, event.Message, event.CreatedAt); err != nil {
			return err
		}
	}

	return nil
}

func matchingAlerts(ctx context.Context, tx *sql.Tx, project string) ([]Alert, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, project, kind, threshold, window_seconds, enabled, created_at
		FROM alerts
		WHERE enabled = 1
		  AND (project = '' OR project = ?)
	`, project)
	if err != nil {
		return nil, fmt.Errorf("list matching observe alerts: %w", err)
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var alert Alert
		var enabled int
		if err := rows.Scan(&alert.ID, &alert.Project, &alert.Kind, &alert.Threshold, &alert.WindowSeconds, &enabled, &alert.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan matching observe alert: %w", err)
		}
		alert.Enabled = enabled == 1
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matching observe alerts: %w", err)
	}

	return alerts, nil
}

func insertAlertDelivery(ctx context.Context, tx *sql.Tx, alert Alert, project, subject, message, createdAt string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO alert_deliveries (alert_id, project, kind, subject, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, alert.ID, project, alert.Kind, subject, message, createdAt)
	if err != nil {
		return fmt.Errorf("insert observe alert delivery: %w", err)
	}

	return nil
}

func execDelete(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("observe cleanup delete: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("observe cleanup count: %w", err)
	}

	return count, nil
}
