package observe

import (
	"context"
	"testing"
	"time"
)

// Contar con los metodos de listado daria un numero que parece cierto y no lo es:
// ListIssues lleva LIMIT 20 por defecto. Esta prueba mete mas issues que ese limite
// justamente para que el atajo falle si alguien lo intenta.
func TestMetricsCountsBeyondTheListLimit(t *testing.T) {
	now := time.Now().UTC()
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	const issues = 35
	for i := 0; i < issues; i++ {
		// El fingerprint enmascara numeros, asi que dos mensajes que solo difieren
		// en un digito son el mismo issue. Hay que variar con palabras.
		event, err := NormalizeEvent("demo", []byte(`{"message":"failure `+word(i)+`","exception_type":"DemoError"}`))
		if err != nil {
			t.Fatalf("NormalizeEvent returned error: %v", err)
		}
		if _, err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("StoreEvent returned error: %v", err)
		}
	}

	snapshot, err := store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}

	total := 0
	for _, count := range snapshot.Issues {
		total += count.Issues
	}
	if total != issues {
		t.Errorf("expected %d issues counted, got %d; the LIMIT of the list methods leaked in", issues, total)
	}
}

// word da palabras distintas por indice. El fingerprint enmascara numeros, correos
// y hashes, asi que "failure 1" y "failure 2" agruparian como un solo issue.
func word(i int) string {
	alphabet := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
		"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey",
		"xray", "yankee", "zulu", "ash", "birch", "cedar", "elm", "fir", "gum",
		"holly", "ivy", "juniper",
	}

	return alphabet[i%len(alphabet)]
}

// Los conteos se agrupan por proyecto y nivel, que son las dos etiquetas seguras:
// conjuntos acotados. Por mensaje o fingerprint la serie explotaria.
func TestMetricsGroupsByProjectAndLevel(t *testing.T) {
	now := time.Now().UTC()
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	payloads := []struct{ project, body string }{
		{"alpha", `{"message":"first alpha failure","level":"error"}`},
		{"alpha", `{"message":"second alpha failure","level":"error"}`},
		{"alpha", `{"message":"an alpha warning","level":"warning"}`},
		{"beta", `{"message":"a beta failure","level":"error"}`},
	}
	for _, p := range payloads {
		event, err := NormalizeEvent(p.project, []byte(p.body))
		if err != nil {
			t.Fatalf("NormalizeEvent returned error: %v", err)
		}
		if _, err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("StoreEvent returned error: %v", err)
		}
	}

	snapshot, err := store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}

	got := map[string]int{}
	for _, count := range snapshot.Issues {
		got[count.Project+"/"+count.Level] = count.Issues
	}
	for key, want := range map[string]int{"alpha/error": 2, "alpha/warning": 1, "beta/error": 1} {
		if got[key] != want {
			t.Errorf("expected %d issues for %s, got %d (all: %#v)", want, key, got[key], snapshot.Issues)
		}
	}
}

// Una corrida cuyo ultimo latido es viejo quedo abierta porque el proceso murio de
// golpe. Contarla como viva daria un uptime que crece solo y no significa nada.
func TestMetricsDoesNotCountAStaleSessionAsRunning(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	if _, err := store.StartCollectorSession(ctx); err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}

	// Latido reciente: la corrida esta viva.
	snapshot, err := store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}
	if !snapshot.CollectorRunning {
		t.Fatal("a session whose heartbeat is fresh should count as running")
	}

	// Media hora sin latir: el proceso murio y nadie cerro la corrida.
	now = now.Add(30 * time.Minute)
	snapshot, err = store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}
	if snapshot.CollectorRunning {
		t.Error("a session with no recent heartbeat must not count as running")
	}
	if snapshot.CollectorUptime != 0 {
		t.Errorf("a dead session has no uptime to report, got %s", snapshot.CollectorUptime)
	}
}

// La metrica que dice si un hueco en las graficas es una aplicacion sana o un
// collector muerto.
func TestMetricsReportsTheCoverageGap(t *testing.T) {
	now := time.Date(2026, 8, 27, 2, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	first, _ := store.StartCollectorSession(ctx)
	now = now.Add(time.Hour)
	if err := store.Heartbeat(ctx, first); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	// Tres horas caido.
	now = now.Add(3 * time.Hour)
	second, _ := store.StartCollectorSession(ctx)
	now = now.Add(time.Hour)
	if err := store.Heartbeat(ctx, second); err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}

	snapshot, err := store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}

	// 22 h y no 3 h: la ventana de la metrica son las ultimas 24 h, y el collector
	// tampoco estaba escuchando en las 19 h anteriores a su primera corrida. Ese
	// hueco inicial **cuenta**, y es lo correcto: son horas del ultimo dia sin
	// nadie recibiendo. Contar solo los huecos entre corridas daria 0 para quien
	// encendio el collector cinco minutos ayer y nada mas desde entonces.
	if want := (22 * time.Hour).Seconds(); snapshot.GapSeconds24h != want {
		t.Errorf("expected %v seconds of gap, got %v", want, snapshot.GapSeconds24h)
	}
}

// El corolario incomodo de contar el hueco inicial: una instalacion nueva reporta
// casi 24 h de hueco durante su primer dia. Es cierto —no habia cobertura— pero
// hay que saberlo antes de poner una alerta encima.
func TestMetricsReportsAFullGapOnAFreshInstall(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	store, _ := clockedStore(t, &now)
	ctx := context.Background()

	if _, err := store.StartCollectorSession(ctx); err != nil {
		t.Fatalf("StartCollectorSession returned error: %v", err)
	}

	snapshot, err := store.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics returned error: %v", err)
	}
	if want := (24 * time.Hour).Seconds(); snapshot.GapSeconds24h != want {
		t.Errorf("a collector started just now has the whole window uncovered: "+
			"expected %v seconds, got %v", want, snapshot.GapSeconds24h)
	}
	// Y aun asi esta corriendo: las dos cosas son ciertas a la vez.
	if !snapshot.CollectorRunning {
		t.Error("the collector is running even though the window is uncovered")
	}
}

// Una base vacia da un snapshot en ceros, no un error. Un scrape que falla y uno
// que devuelve cero son cosas distintas y Prometheus las trata distinto.
func TestMetricsOnAnEmptyDatabase(t *testing.T) {
	now := time.Now().UTC()
	store, _ := clockedStore(t, &now)

	snapshot, err := store.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics on an empty database returned error: %v", err)
	}
	if len(snapshot.Issues) != 0 || len(snapshot.Containers) != 0 {
		t.Errorf("expected empty counts, got %#v", snapshot)
	}
	if snapshot.CollectorRunning {
		t.Error("no sessions recorded means unknown, not running")
	}
}
