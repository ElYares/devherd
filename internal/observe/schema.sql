PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'new',
    level TEXT NOT NULL DEFAULT 'error',
    platform TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    container TEXT NOT NULL DEFAULT '',
    exception_type TEXT NOT NULL DEFAULT '',
    culprit TEXT NOT NULL DEFAULT '',
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project, fingerprint)
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    project TEXT NOT NULL,
    issue_id INTEGER NOT NULL,
    timestamp TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT 'error',
    platform TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    container TEXT NOT NULL DEFAULT '',
    exception_type TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    culprit TEXT NOT NULL DEFAULT '',
    transaction_name TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL DEFAULT 'local',
    release TEXT NOT NULL DEFAULT '',
    raw_payload TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_observe_issues_project ON issues(project, last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_observe_issues_status ON issues(status);
CREATE INDEX IF NOT EXISTS idx_observe_events_project ON events(project, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_observe_events_issue_id ON events(issue_id);

CREATE TABLE IF NOT EXISTS containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    container_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    restart_count INTEGER NOT NULL DEFAULT 0,
    labels_json TEXT NOT NULL DEFAULT '{}',
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS container_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    container_id TEXT NOT NULL,
    name TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    restart_count INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS container_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT '',
    container TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL DEFAULT '',
    stream TEXT NOT NULL DEFAULT 'combined',
    message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_observe_containers_project ON containers(project, service);
CREATE INDEX IF NOT EXISTS idx_observe_container_events_project ON container_events(project, service, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_observe_container_logs_event_id ON container_logs(event_id);

CREATE TABLE IF NOT EXISTS alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    threshold INTEGER NOT NULL DEFAULT 1,
    window_seconds INTEGER NOT NULL DEFAULT 300,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alert_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_id INTEGER NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    subject TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(alert_id) REFERENCES alerts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_observe_alerts_project ON alerts(project, kind);
CREATE INDEX IF NOT EXISTS idx_observe_alert_deliveries_project ON alert_deliveries(project, created_at DESC);
