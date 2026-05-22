package observe

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

var panelTemplate = template.Must(template.New("observe-panel").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DevHerd Observe</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #182026;
      --muted: #66717d;
      --line: #d7dde3;
      --accent: #0f766e;
      --danger: #b42318;
      --warn: #b54708;
      --ok: #067647;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: 14px/1.4 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    header {
      height: 56px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 24px;
      border-bottom: 1px solid var(--line);
      background: var(--panel);
      position: sticky;
      top: 0;
      z-index: 2;
    }
    h1 { font-size: 18px; margin: 0; font-weight: 650; }
    main { padding: 18px 24px 28px; max-width: 1440px; margin: 0 auto; }
    .toolbar {
      display: flex;
      gap: 10px;
      align-items: center;
      margin-bottom: 16px;
    }
    input, button {
      height: 34px;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 6px;
      padding: 0 10px;
      font: inherit;
    }
    button {
      cursor: pointer;
      background: var(--accent);
      color: #fff;
      border-color: var(--accent);
      font-weight: 600;
    }
    .grid {
      display: grid;
      grid-template-columns: minmax(0, 1.3fr) minmax(360px, .7fr);
      gap: 16px;
      align-items: start;
    }
    section {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
      margin-bottom: 16px;
    }
    section h2 {
      margin: 0;
      padding: 11px 14px;
      border-bottom: 1px solid var(--line);
      font-size: 13px;
      text-transform: uppercase;
      letter-spacing: 0;
      color: var(--muted);
    }
    table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
    }
    th, td {
      padding: 9px 10px;
      border-bottom: 1px solid #edf0f3;
      text-align: left;
      vertical-align: top;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    th { color: var(--muted); font-size: 12px; font-weight: 600; }
    tr:last-child td { border-bottom: 0; }
    .pill {
      display: inline-block;
      min-width: 42px;
      text-align: center;
      padding: 2px 7px;
      border-radius: 999px;
      background: #eef4f4;
      color: var(--accent);
      font-size: 12px;
      font-weight: 650;
    }
    .danger { color: var(--danger); }
    .warn { color: var(--warn); }
    .ok { color: var(--ok); }
    .muted { color: var(--muted); }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .empty { padding: 14px; color: var(--muted); }
    .timeline { padding: 12px 14px; }
    .timeline pre {
      white-space: pre-wrap;
      word-break: break-word;
      margin: 8px 0 0;
      padding: 10px;
      border-radius: 6px;
      background: #f3f5f7;
      max-height: 420px;
      overflow: auto;
    }
    @media (max-width: 960px) {
      header { padding: 0 14px; }
      main { padding: 14px; }
      .grid { grid-template-columns: 1fr; }
      .toolbar { flex-wrap: wrap; }
      input { flex: 1 1 220px; }
    }
  </style>
</head>
<body>
  <header>
    <h1>DevHerd Observe</h1>
    <span class="muted mono">local</span>
  </header>
  <main>
    <div class="toolbar">
      <input id="project" placeholder="Project filter">
      <button id="refresh" type="button">Refresh</button>
      <span id="status" class="muted"></span>
    </div>
    <div class="grid">
      <div>
        <section>
          <h2>Issues</h2>
          <div id="issues"></div>
        </section>
        <section>
          <h2>Events</h2>
          <div id="events"></div>
        </section>
      </div>
      <div>
        <section>
          <h2>Alerts</h2>
          <div id="alerts"></div>
        </section>
        <section>
          <h2>Containers</h2>
          <div id="containers"></div>
        </section>
        <section>
          <h2>Timeline</h2>
          <div id="timeline" class="timeline muted">Select an event.</div>
        </section>
      </div>
    </div>
  </main>
  <script>
    const $ = (id) => document.getElementById(id);

    function esc(value) {
      return String(value ?? '').replace(/[&<>"']/g, (c) => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
      }[c]));
    }

    async function getJSON(path) {
      const res = await fetch(path);
      if (!res.ok) throw new Error(await res.text());
      return await res.json();
    }

    function projectQuery() {
      const project = $('project').value.trim();
      return project ? '&project=' + encodeURIComponent(project) : '';
    }

    async function load() {
      $('status').textContent = 'loading';
      try {
        const q = projectQuery();
        const [issues, events, containers, alerts] = await Promise.all([
          getJSON('/api/observe/issues?limit=25' + q),
          getJSON('/api/observe/events?limit=25' + q),
          getJSON('/api/observe/containers?limit=25' + q),
          getJSON('/api/observe/alerts?limit=25' + q),
        ]);
        renderIssues(issues);
        renderEvents(events);
        renderContainers(containers);
        renderAlerts(alerts);
        $('status').textContent = 'updated ' + new Date().toLocaleTimeString();
      } catch (err) {
        $('status').textContent = 'error';
        console.error(err);
      }
    }

    function renderIssues(items) {
      if (!items.length) return $('issues').innerHTML = '<div class="empty">No issues.</div>';
      $('issues').innerHTML = '<table><thead><tr><th style="width:58px">ID</th><th>Title</th><th style="width:72px">Count</th><th style="width:100px">Service</th><th style="width:130px">Last seen</th></tr></thead><tbody>' +
        items.map(i => '<tr><td class="mono">' + i.id + '</td><td title="' + esc(i.title) + '">' + esc(i.title) + '</td><td><span class="pill">' + i.event_count + '</span></td><td>' + esc(i.service) + '</td><td class="muted">' + esc(i.last_seen) + '</td></tr>').join('') +
        '</tbody></table>';
    }

    function renderEvents(items) {
      if (!items.length) return $('events').innerHTML = '<div class="empty">No events.</div>';
      $('events').innerHTML = '<table><thead><tr><th style="width:90px">Event</th><th>Message</th><th style="width:110px">Project</th><th style="width:100px">Service</th><th style="width:110px">Container</th></tr></thead><tbody>' +
        items.map(e => '<tr><td class="mono"><a href="#" data-event="' + esc(e.event_id) + '">' + esc(e.event_id.slice(0, 8)) + '</a></td><td title="' + esc(e.message) + '">' + esc(e.message) + '</td><td>' + esc(e.project) + '</td><td>' + esc(e.service) + '</td><td>' + esc(e.container) + '</td></tr>').join('') +
        '</tbody></table>';
      document.querySelectorAll('a[data-event]').forEach(el => el.addEventListener('click', (ev) => {
        ev.preventDefault();
        loadTimeline(el.dataset.event);
      }));
    }

    function renderContainers(items) {
      if (!items.length) return $('containers').innerHTML = '<div class="empty">No containers.</div>';
      $('containers').innerHTML = '<table><thead><tr><th>Container</th><th style="width:84px">Service</th><th style="width:82px">Status</th><th style="width:52px">Restarts</th></tr></thead><tbody>' +
        items.map(c => '<tr><td title="' + esc(c.name) + '">' + esc(c.name) + '</td><td>' + esc(c.service) + '</td><td class="' + (c.status === 'running' ? 'ok' : 'danger') + '">' + esc(c.status) + '</td><td>' + c.restart_count + '</td></tr>').join('') +
        '</tbody></table>';
    }

    function renderAlerts(items) {
      if (!items.length) return $('alerts').innerHTML = '<div class="empty">No alert deliveries.</div>';
      $('alerts').innerHTML = '<table><thead><tr><th>Subject</th><th style="width:110px">Project</th><th style="width:95px">Kind</th></tr></thead><tbody>' +
        items.map(a => '<tr><td title="' + esc(a.message) + '">' + esc(a.subject) + '</td><td>' + esc(a.project) + '</td><td>' + esc(a.kind) + '</td></tr>').join('') +
        '</tbody></table>';
    }

    async function loadTimeline(eventId) {
      const data = await getJSON('/api/observe/timeline?event_id=' + encodeURIComponent(eventId));
      const lines = [
        'event: ' + data.event.event_id,
        'project: ' + data.event.project,
        'service: ' + (data.event.service || ''),
        'container: ' + (data.event.container || ''),
        'message: ' + (data.event.message || ''),
        '',
        'container events:',
        ...(data.container_events || []).map(e => '- ' + e.created_at + ' ' + e.kind + ' ' + e.message),
        '',
        'logs:',
        ...(data.logs || []).map(l => '- ' + l.timestamp + ' ' + l.message)
      ];
      $('timeline').innerHTML = '<pre>' + esc(lines.join('\n')) + '</pre>';
    }

    $('refresh').addEventListener('click', load);
    $('project').addEventListener('keydown', (ev) => { if (ev.key === 'Enter') load(); });
    load();
  </script>
</body>
</html>`))

func (s Server) handlePanel(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/observe" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = panelTemplate.Execute(w, nil)
}

func (s Server) handlePanelAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/observe/"), "/")
	project := strings.TrimSpace(r.URL.Query().Get("project"))
	limit := queryInt(r, "limit", 25)

	switch action {
	case "issues":
		issues, err := s.store.ListIssues(r.Context(), project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, issues)
	case "events":
		events, err := s.store.ListEvents(r.Context(), project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, events)
	case "containers":
		containers, err := s.store.ListContainers(r.Context(), project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, containers)
	case "alerts":
		deliveries, err := s.store.ListAlertDeliveries(r.Context(), project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, deliveries)
	case "timeline":
		eventID := strings.TrimSpace(r.URL.Query().Get("event_id"))
		if eventID == "" {
			writeError(w, http.StatusBadRequest, "event_id is required")
			return
		}
		timeline, err := s.store.Timeline(r.Context(), eventID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, timeline)
	default:
		writeError(w, http.StatusNotFound, "unsupported observe panel endpoint")
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
