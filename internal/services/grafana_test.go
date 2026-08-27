package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Los archivos de provisioning van en las rutas que el compose monta. Una ruta
// mal puesta no falla al arrancar: Grafana levanta igual y muestra un tablero
// vacio, que es la peor forma de fallar porque parece que funciona.
func TestGrafanaProvisionsDatasourceAndDashboard(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	expected := []string{
		"grafana/alerting/devherd-rules.yml",
		"grafana/dashboards/devherd.yml",
		"grafana/dashboards/devherd/devherd-observe.json",
		"grafana/datasources/prometheus.yml",
	}
	if len(paths) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, paths)
	}
	for i, want := range expected {
		if paths[i] != want {
			t.Errorf("file %d: expected %q, got %q", i, want, paths[i])
		}
	}
}

// Se apunta a Prometheus por su alias de red, no por IP: las dos comparten
// infra_net y Docker resuelve el nombre. Una IP cambiaria al recrear la red, que
// es la misma trampa que ya costo un falso positivo en Observe.
func TestGrafanaDatasourceUsesTheNetworkAlias(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var datasource string
	for _, file := range files {
		if strings.HasSuffix(file.Path, "datasources/prometheus.yml") {
			datasource = file.Content
		}
	}
	if datasource == "" {
		t.Fatal("the datasource file is missing")
	}

	if !strings.Contains(datasource, "url: http://prometheus:9090") {
		t.Errorf("the datasource should target the network alias, got:\n%s", datasource)
	}
	// Una IP en el datasource es la forma de que deje de funcionar sola.
	for _, line := range strings.Split(datasource, "\n") {
		if strings.Contains(line, "url:") && strings.Contains(line, "172.") {
			t.Errorf("the datasource must not hardcode an IP: %q", line)
		}
	}
}

// **El criterio que decide si empaquetar Grafana valio la pena.** Con datasource y
// sin tableros, el usuario se queda exactamente donde estaba.
func TestGrafanaDashboardQueriesTheObserveMetrics(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var raw string
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".json") {
			raw = file.Content
		}
	}
	if raw == "" {
		t.Fatal("the dashboard is missing")
	}

	var dashboard struct {
		UID    string `json:"uid"`
		Title  string `json:"title"`
		Panels []struct {
			Title   string `json:"title"`
			Targets []struct {
				Expr       string `json:"expr"`
				Datasource struct {
					UID string `json:"uid"`
				} `json:"datasource"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(raw), &dashboard); err != nil {
		t.Fatalf("the dashboard is not valid JSON: %v", err)
	}

	if len(dashboard.Panels) == 0 {
		t.Fatal("a dashboard with no panels leaves the user exactly where they were")
	}

	// Cada panel consulta algo, y contra el datasource que se provisiona.
	queries := make([]string, 0, len(dashboard.Panels))
	for _, panel := range dashboard.Panels {
		if len(panel.Targets) == 0 {
			t.Errorf("panel %q has no query", panel.Title)

			continue
		}
		for _, target := range panel.Targets {
			if target.Expr == "" {
				t.Errorf("panel %q has an empty query", panel.Title)
			}
			if target.Datasource.UID != "devherd-prometheus" {
				t.Errorf("panel %q points at %q instead of the provisioned datasource",
					panel.Title, target.Datasource.UID)
			}
			queries = append(queries, target.Expr)
		}
	}

	// Las tres familias que cuentan la historia: donde duele, que se reinicia, y si
	// el collector estuvo escuchando.
	joined := strings.Join(queries, " ")
	for _, metric := range []string{
		"devherd_observe_issues",
		"devherd_observe_container_restarts_total",
		"devherd_observe_collector_gap_seconds",
	} {
		if !strings.Contains(joined, metric) {
			t.Errorf("no panel queries %s", metric)
		}
	}
}

// Grafana sin Prometheus arranca y muestra paneles vacios. Declararlo permite
// avisar antes, en vez de que se descubra al abrir el tablero.
func TestGrafanaDependsOnPrometheus(t *testing.T) {
	if got := DependsOn("grafana"); got != "prometheus" {
		t.Errorf("expected grafana to depend on prometheus, got %q", got)
	}
	for _, service := range []string{"redis", "mailpit", "prometheus"} {
		if got := DependsOn(service); got != "" {
			t.Errorf("%s should not declare a dependency, got %q", service, got)
		}
	}
}

// Arrancarlo escribe los archivos donde el compose los monta.
func TestStartGrafanaWritesItsProvisioning(t *testing.T) {
	m := newTestManager(t, &fakeRunner{outputs: []string{"", "", ""}})

	_, files, err := m.Start(context.Background(), "grafana", StartOptions{})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("expected four provisioning files, got %#v", files)
	}

	for _, relative := range []string{
		"grafana/datasources/prometheus.yml",
		"grafana/dashboards/devherd.yml",
		"grafana/dashboards/devherd/devherd-observe.json",
		"grafana/alerting/devherd-rules.yml",
	} {
		if _, err := os.Stat(filepath.Join(m.dir, filepath.FromSlash(relative))); err != nil {
			t.Errorf("%s was not written: %v", relative, err)
		}
	}
}

// Sin login: es un entorno local y un login que nadie recuerda es friccion sin
// seguridad. El puerto solo escucha en loopback, asi que no hay superficie que
// proteger.
func TestGrafanaDoesNotAskForCredentials(t *testing.T) {
	for _, want := range []string{
		`GF_AUTH_ANONYMOUS_ENABLED: "true"`,
		`GF_AUTH_DISABLE_LOGIN_FORM: "true"`,
	} {
		if !strings.Contains(composeContent, want) {
			t.Errorf("the compose should set %s", want)
		}
	}
	if !strings.Contains(composeContent, `"127.0.0.1:3000:3000"`) {
		t.Error("grafana must publish its port on loopback only; anonymous access depends on it")
	}
}

// Grafana entra al catalogo sin desplazar a nadie.
func TestGrafanaIsPartOfTheCatalog(t *testing.T) {
	supported := SupportedServices()
	for _, want := range []string{"redis", "mailpit", "prometheus", "grafana"} {
		if !contains(supported, want) {
			t.Errorf("%s should be a supported service, got %v", want, supported)
		}
	}
}

// **El caso que tumba el servicio.** Sin webhook, el contact point NO se escribe:
// un $__env{} sin definir resuelve a cadena vacia, la validacion del receptor pide
// un `url` que ya no esta, y Grafana sale con codigo 1. Verificado contra
// grafana/grafana:11.5.1, que muere con "failure parsing contact points:
// required field 'url' is not specified".
//
// Un servicio compartido no puede caerse porque alguien no use Slack.
func TestGrafanaOmitsTheSlackContactPointWithoutAWebhook(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{SlackConfigured: false})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	for _, file := range files {
		if strings.Contains(file.Path, "devherd-slack") {
			t.Fatalf("the Slack contact point must not be written without a webhook: %s", file.Path)
		}
	}

	// Las reglas si van: sin destino no suenan, pero se ven disparar en la
	// interfaz, y eso ya es mas de lo que da un tablero que hay que estar mirando.
	if !hasPath(files, "grafana/alerting/devherd-rules.yml") {
		t.Error("the alert rules should be provisioned even without Slack")
	}
}

// Con webhook si se escribe, y toma el secreto del entorno en vez de llevarlo
// dentro. El archivo es una plantilla de DevHerd; el webhook es del usuario.
func TestGrafanaWritesTheSlackContactPointWithAWebhook(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{SlackConfigured: true})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var slack string
	for _, file := range files {
		if file.Path == "grafana/alerting/devherd-slack.yml" {
			slack = file.Content
		}
	}
	if slack == "" {
		t.Fatal("the Slack contact point is missing")
	}

	if !strings.Contains(slack, "url: $__env{DEVHERD_SLACK_WEBHOOK}") {
		t.Error("the webhook must come from the environment, not from the file")
	}
	// Un webhook literal en una plantilla versionada es un secreto filtrado. Se
	// miran solo las lineas efectivas: los comentarios documentan la forma que
	// tiene que tener el valor, y ahi el ejemplo es justo lo util.
	for _, line := range strings.Split(slack, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.Contains(line, "hooks.slack.com") {
			t.Errorf("the template must not carry a real webhook: %q", line)
		}
	}
}

// El .env es la fuente de verdad, igual que para el token de Jupyter.
func TestSlackConfiguredReadsTheEnvFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		want bool
	}{
		{"sin archivo", "", false},
		{"definido", "DEVHERD_SLACK_WEBHOOK=https://hooks.slack.com/services/T/B/x\n", true},
		{"vacio", "DEVHERD_SLACK_WEBHOOK=\n", false},
		{"solo espacios", "DEVHERD_SLACK_WEBHOOK=   \n", false},
		// Comentarlo es como se desactiva un webhook sin borrarlo. Tomarlo por
		// bueno escribiria el contact point y dejaria a Grafana sin arrancar.
		{"comentado", "#DEVHERD_SLACK_WEBHOOK=https://hooks.slack.com/services/T/B/x\n", false},
		{"entre otras variables", "JUPYTER_TOKEN=abc\nDEVHERD_SLACK_WEBHOOK=https://h/x\nDEVHERD_UID=1000\n", true},
		// Un prefijo que solo se parece no cuenta.
		{"nombre parecido", "DEVHERD_SLACK_WEBHOOK_OLD=https://h/x\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.env != "" {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tc.env), 0o644); err != nil {
					t.Fatalf("write .env: %v", err)
				}
			}

			if got := SlackConfigured(dir); got != tc.want {
				t.Errorf("SlackConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Las reglas consultan las series que Observe publica de verdad. Una regla contra
// una metrica que no existe no falla: se queda en NoData para siempre, que se ve
// igual que "todo bien".
func TestGrafanaAlertRulesQueryTheRealMetrics(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var rules string
	for _, file := range files {
		if file.Path == "grafana/alerting/devherd-rules.yml" {
			rules = file.Content
		}
	}
	if rules == "" {
		t.Fatal("the alert rules are missing")
	}

	for _, metric := range []string{
		"devherd_observe_collector_uptime_seconds",
		"devherd_observe_container_restarts_total",
		"devherd_observe_events_total",
	} {
		if !strings.Contains(rules, metric) {
			t.Errorf("no rule queries %s", metric)
		}
	}

	// **`collector_gap_seconds` no puede ser un disparador.** Es el total de una
	// ventana movil de 24 h: recien instalado vale casi 24 h y cualquier umbral
	// dispara el primer dia diciendo solo "acabas de instalar esto". Mirar su
	// crecimiento tampoco sirve, porque la ventana drena la historia vieja a un
	// segundo por segundo y esconde los cortes reales debajo. Vive en el tablero,
	// que es donde una metrica de ventana se lee bien.
	if strings.Contains(rules, "devherd_observe_collector_gap_seconds") {
		t.Error("the 24h gap window fires on day one for every new install; alert on collector restarts instead")
	}

	// Contra el datasource que DevHerd provisiona, no contra el que Grafana
	// elija por defecto.
	if !strings.Contains(rules, "datasourceUid: devherd-prometheus") {
		t.Error("the rules must target the provisioned datasource")
	}

	// La etiqueta que la ruta de Slack matchea. Sin ella las alertas se evaluan
	// y no se entregan, que es la falla silenciosa de siempre.
	if !strings.Contains(rules, `devherd: "true"`) {
		t.Error("the rules must carry the devherd label the Slack route matches on")
	}
}

// El collector caido se detecta con absent(), y absent() **no devuelve nada**
// cuando la serie esta. Sin noDataState: OK, el estado sano se leeria como
// NoData y la alerta viviria disparada al reves.
func TestCollectorDownRuleTreatsNoDataAsHealthy(t *testing.T) {
	files, err := ServiceFiles("grafana", ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceFiles returned error: %v", err)
	}

	var rules string
	for _, file := range files {
		if file.Path == "grafana/alerting/devherd-rules.yml" {
			rules = file.Content
		}
	}

	if !strings.Contains(rules, "absent(devherd_observe_collector_uptime_seconds)") {
		t.Error("the collector-down rule should use absent(); the metric is omitted, not zeroed, when it is not running")
	}
	// Cada regla declara su noDataState. Ninguna puede quedarse con el default.
	rulesCount := strings.Count(rules, "- uid: devherd-")
	noData := strings.Count(rules, "noDataState:")
	if rulesCount != noData {
		t.Errorf("%d rules but %d noDataState declarations; every rule must declare one", rulesCount, noData)
	}
}

// El compose tiene que montar el directorio y pasar la variable, o el
// provisioning que DevHerd escribe no lo ve nadie.
func TestComposeMountsAlertingAndPassesTheWebhook(t *testing.T) {
	if !strings.Contains(composeContent, "./grafana/alerting:/etc/grafana/provisioning/alerting:ro") {
		t.Error("the compose must mount the alerting provisioning directory")
	}
	if !strings.Contains(composeContent, "DEVHERD_SLACK_WEBHOOK: ${DEVHERD_SLACK_WEBHOOK:-") {
		t.Error("the compose must pass the webhook through to Grafana")
	}
	// **El default no puede ser vacio.** Un $__env{} vacio mata a Grafana al
	// arrancar; una URL invalida solo deja las notificaciones mudas. Si alguien
	// borra el webhook del .env con el contact point ya escrito, la diferencia es
	// entre un Slack callado y un Grafana caido.
	if strings.Contains(composeContent, "DEVHERD_SLACK_WEBHOOK: ${DEVHERD_SLACK_WEBHOOK:-}") {
		t.Error("an empty default stops Grafana from starting; use an invalid URL instead")
	}
}

func hasPath(files []ManagedFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}

	return false
}
