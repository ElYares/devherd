package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/config"
)

// fakeRunner registra las invocaciones y devuelve respuestas predefinidas.
type fakeRunner struct {
	calls   [][]string
	outputs []string
	errs    []error
	idx     int
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)

	var out string
	var err error
	if f.idx < len(f.outputs) {
		out = f.outputs[f.idx]
	}
	if f.idx < len(f.errs) {
		err = f.errs[f.idx]
	}
	f.idx++
	return out, err
}

func newTestManager(t *testing.T, r *fakeRunner) Manager {
	t.Helper()
	return NewManagerWithRunner(config.Paths{ComposeDir: t.TempDir()}, r)
}

func TestStartEnsuresNetworkAndComposesUp(t *testing.T) {
	// network inspect OK (red ya existe) → 2 llamadas: inspect + compose up.
	r := &fakeRunner{outputs: []string{"net", "started"}}
	m := newTestManager(t, r)

	out, err := m.Start(context.Background(), "redis")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if out != "started" {
		t.Errorf("output = %q, want %q", out, "started")
	}
	if len(r.calls) != 2 {
		t.Fatalf("expected 2 docker calls, got %d: %v", len(r.calls), r.calls)
	}
	if got := strings.Join(r.calls[0], " "); got != "docker network inspect "+NetworkName {
		t.Errorf("first call = %q", got)
	}
	if up := strings.Join(r.calls[1], " "); !strings.Contains(up, "compose") || !strings.HasSuffix(up, "up -d redis") {
		t.Errorf("second call = %q, want a compose up -d redis", up)
	}
}

func TestStartCreatesNetworkWhenInspectFails(t *testing.T) {
	// inspect falla → network create, luego compose up.
	r := &fakeRunner{
		outputs: []string{"", "created", "started"},
		errs:    []error{errors.New("no such network"), nil, nil},
	}
	m := newTestManager(t, r)

	if _, err := m.Start(context.Background(), "mailpit"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(r.calls) != 3 {
		t.Fatalf("expected 3 docker calls, got %d: %v", len(r.calls), r.calls)
	}
	if create := strings.Join(r.calls[1], " "); !strings.Contains(create, "network create") || !strings.Contains(create, "devherd.managed=true") {
		t.Errorf("expected a managed network create, got %q", create)
	}
}

func TestStartRejectsUnsupportedServiceWithoutDocker(t *testing.T) {
	r := &fakeRunner{}
	m := newTestManager(t, r)

	if _, err := m.Start(context.Background(), "postgres"); err == nil {
		t.Fatal("expected error for unsupported service")
	}
	if len(r.calls) != 0 {
		t.Errorf("no docker calls expected for invalid service, got %v", r.calls)
	}
}

func TestStopComposesStop(t *testing.T) {
	r := &fakeRunner{outputs: []string{"stopped"}}
	m := newTestManager(t, r)

	if _, err := m.Stop(context.Background(), "redis"); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 docker call, got %d: %v", len(r.calls), r.calls)
	}
	if got := strings.Join(r.calls[0], " "); !strings.HasSuffix(got, "stop redis") {
		t.Errorf("call = %q, want compose stop redis", got)
	}
}

func TestStatusAllServices(t *testing.T) {
	r := &fakeRunner{outputs: []string{"ps output"}}
	m := newTestManager(t, r)

	if _, err := m.Status(context.Background(), ""); err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if got := strings.Join(r.calls[0], " "); !strings.HasSuffix(got, "ps") {
		t.Errorf("call = %q, want compose ps", got)
	}
}
