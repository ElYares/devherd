package coverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// projectWith arma un proyecto con los reportes indicados. El contenido no
// importa para el descubrimiento —eso lo decide Parse— pero no puede estar vacio:
// un archivo de cero bytes no cuenta como reporte.
func projectWith(t *testing.T, files ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("mode: set\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return root
}

// Cada stack tiene su convencion, y el descubrimiento la sigue sin que nadie
// tenga que recordarla.
func TestDiscoverReportFindsTheConventionalPathPerStack(t *testing.T) {
	cases := []struct {
		stack string
		file  string
	}{
		{"go", "coverage.out"},
		{"laravel", "coverage/clover.xml"},
		{"vue", "coverage/lcov.info"},
		{"node", "coverage/lcov.info"},
		{"python", "coverage.xml"},
		{"flask", "coverage.xml"},
	}

	for _, tc := range cases {
		t.Run(tc.stack, func(t *testing.T) {
			root := projectWith(t, tc.file)

			discovery, err := DiscoverReport(root, tc.stack)
			if err != nil {
				t.Fatalf("DiscoverReport returned error: %v", err)
			}
			if discovery.Chosen.RelPath != tc.file {
				t.Errorf("expected %q, got %q", tc.file, discovery.Chosen.RelPath)
			}
			if discovery.Chosen.Stack != tc.stack {
				t.Errorf("expected the candidate to name stack %q, got %q", tc.stack, discovery.Chosen.Stack)
			}
		})
	}
}

// Con dos reportes de formatos distintos gana el del stack detectado, y el otro
// se menciona. Tomar el primero que aparezca es como se lee el reporte del front
// creyendo que es el del back.
func TestDiscoverReportPrefersTheDetectedStackAndMentionsTheRest(t *testing.T) {
	root := projectWith(t, "coverage.out", "coverage/lcov.info")

	discovery, err := DiscoverReport(root, "go")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}

	if discovery.Chosen.RelPath != "coverage.out" {
		t.Errorf("expected the Go profile to win, got %q", discovery.Chosen.RelPath)
	}
	if len(discovery.Others) != 1 || discovery.Others[0].RelPath != "coverage/lcov.info" {
		t.Fatalf("expected the lcov report to be mentioned, got %#v", discovery.Others)
	}
	// Se nombra de que stack es, para que quede claro por que no se eligio.
	if discovery.Others[0].Stack == "" {
		t.Error("the mentioned report should name the stack whose convention placed it")
	}
}

// El reporte del proyecto manda sobre el que deja --run: el administrado puede
// ser de una corrida vieja.
func TestDiscoverReportPrefersTheProjectReportOverTheManagedOne(t *testing.T) {
	root := projectWith(t, "coverage.out", ManagedReportPrefix+".out")

	discovery, err := DiscoverReport(root, "go")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}

	if discovery.Chosen.RelPath != "coverage.out" {
		t.Errorf("expected the project report to win, got %q", discovery.Chosen.RelPath)
	}
	if discovery.Chosen.Managed {
		t.Error("the chosen report should not be the managed one")
	}
	if len(discovery.Others) != 1 || !discovery.Others[0].Managed {
		t.Fatalf("expected the managed report to be mentioned, got %#v", discovery.Others)
	}
}

// Si lo unico que hay es el reporte de --run, se usa: es un reporte valido, solo
// que puede ser viejo, y por eso viene marcado.
func TestDiscoverReportFallsBackToTheManagedReport(t *testing.T) {
	root := projectWith(t, ManagedReportPrefix+".out")

	discovery, err := DiscoverReport(root, "go")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}

	if !discovery.Chosen.Managed {
		t.Errorf("expected the managed report to be chosen and marked, got %#v", discovery.Chosen)
	}
}

// Sin reporte, el mensaje dice donde se busco y como generar uno. "No encontrado"
// a secas deja al usuario sin siguiente paso.
func TestDiscoverReportErrorNamesThePathsAndTheFix(t *testing.T) {
	root := projectWith(t)

	_, err := DiscoverReport(root, "go")
	if err == nil {
		t.Fatal("expected an error when there is no report")
	}

	var missing ErrNoReportFound
	if !asErrNoReportFound(err, &missing) {
		t.Fatalf("expected ErrNoReportFound, got %T", err)
	}
	if len(missing.Searched) == 0 {
		t.Error("the error should carry the paths that were searched")
	}

	message := err.Error()
	for _, expected := range []string{"coverage.out", "looked for", "go test ./...", "devherd coverage --run"} {
		if !strings.Contains(message, expected) {
			t.Errorf("expected the message to contain %q, got:\n%s", expected, message)
		}
	}
}

// Un stack desconocido no bloquea el descubrimiento: se buscan todas las
// convenciones y se dice de cual salio lo que se encontro.
func TestDiscoverReportSearchesEveryConventionForAnUnknownStack(t *testing.T) {
	root := projectWith(t, "coverage/lcov.info")

	discovery, err := DiscoverReport(root, "")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}
	if discovery.Chosen.RelPath != "coverage/lcov.info" {
		t.Errorf("expected the lcov report, got %q", discovery.Chosen.RelPath)
	}
	if discovery.Chosen.Stack == "" {
		t.Error("the candidate should name the stack whose convention placed it")
	}
}

// Un archivo de cero bytes lo dejan las herramientas que murieron antes de
// escribir. Elegirlo daria un "formato desconocido" que manda a buscar el
// problema al lugar equivocado.
func TestDiscoverReportIgnoresEmptyFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "coverage.out"), nil, 0o644); err != nil {
		t.Fatalf("write empty report: %v", err)
	}

	if _, err := DiscoverReport(root, "go"); err == nil {
		t.Fatal("expected a zero-byte report to be ignored")
	}
}

// Un directorio llamado como un reporte no es un reporte.
func TestDiscoverReportIgnoresDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "coverage.out"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, err := DiscoverReport(root, "go"); err == nil {
		t.Fatal("expected a directory not to be taken as a report")
	}
}

// Dos corridas sobre el mismo proyecto tienen que elegir el mismo archivo: el
// recorrido de un mapa en Go no tiene orden, y sin fijarlo la eleccion cambiaria
// entre invocaciones.
func TestDiscoverReportIsDeterministic(t *testing.T) {
	root := projectWith(t, "coverage.xml", "coverage/lcov.info", "coverage.out")

	first, err := DiscoverReport(root, "")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}
	for i := 0; i < 20; i++ {
		again, err := DiscoverReport(root, "")
		if err != nil {
			t.Fatalf("DiscoverReport returned error: %v", err)
		}
		if again.Chosen.RelPath != first.Chosen.RelPath {
			t.Fatalf("discovery is not deterministic: got %q then %q",
				first.Chosen.RelPath, again.Chosen.RelPath)
		}
	}
}

func asErrNoReportFound(err error, target *ErrNoReportFound) bool {
	missing, ok := err.(ErrNoReportFound)
	if ok {
		*target = missing
	}

	return ok
}

// Una cobertura vieja leida como actual es peor que no tenerla: da por probado
// codigo que puede haber cambiado entero desde entonces.
func TestCandidateIsStaleAfterSevenDays(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		age   time.Duration
		stale bool
	}{
		{"recien escrito", 0, false},
		{"seis dias", 6 * 24 * time.Hour, false},
		{"justo siete dias", StaleReportAge, false},
		{"siete dias y un minuto", StaleReportAge + time.Minute, true},
		{"un mes", 30 * 24 * time.Hour, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := Candidate{Path: "coverage.out", ModTime: now.Add(-tc.age)}
			if candidate.IsStale(now) != tc.stale {
				t.Errorf("expected IsStale()=%v for an age of %s, got %v",
					tc.stale, tc.age, candidate.IsStale(now))
			}
		})
	}
}

// Sin fecha no se inventa una antiguedad: un candidato sin ModTime no es viejo,
// es desconocido, y avisar de algo que no se sabe es ruido.
func TestCandidateWithoutModTimeIsNeverStale(t *testing.T) {
	if (Candidate{Path: "coverage.out"}).IsStale(time.Now()) {
		t.Error("a candidate with no mod time should not be reported as stale")
	}
}

// El descubrimiento anota cuando se escribio cada reporte, que es de donde sale
// el aviso.
func TestDiscoverReportRecordsTheModTime(t *testing.T) {
	root := projectWith(t, "coverage.out")
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "coverage.out"), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	discovery, err := DiscoverReport(root, "go")
	if err != nil {
		t.Fatalf("DiscoverReport returned error: %v", err)
	}
	if discovery.Chosen.ModTime.IsZero() {
		t.Fatal("the candidate should carry the file's mod time")
	}
	if !discovery.Chosen.IsStale(time.Now()) {
		t.Errorf("a 30-day-old report should be stale, got an age of %s",
			discovery.Chosen.Age(time.Now()))
	}
}

// Un reporte pasado con --report tambien tiene edad, y engaña igual.
func TestCandidateForRecordsTheModTimeOfAnExplicitReport(t *testing.T) {
	root := projectWith(t, "somewhere/else.out")
	path := filepath.Join(root, "somewhere", "else.out")
	old := time.Now().Add(-14 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	candidate, err := CandidateFor(path)
	if err != nil {
		t.Fatalf("CandidateFor returned error: %v", err)
	}
	if !candidate.IsStale(time.Now()) {
		t.Error("a 14-day-old explicit report should be stale")
	}
	if candidate.Managed {
		t.Error("a report that is not named .devherd.coverage.* is not managed")
	}
}
