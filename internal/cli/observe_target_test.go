package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/observe"
	"github.com/spf13/cobra"
)

func TestResolveObserveTargetFindsRegisteredProjectByName(t *testing.T) {
	root := newComposeProjectDir(t)
	app := newTestAppContext(t, config.Default())
	seedProject(t, app.DB, root)

	target, err := resolveObserveTarget(context.Background(), app, "registrado")
	if err != nil {
		t.Fatalf("resolveObserveTarget returned error: %v", err)
	}
	if target.Name != "registrado" {
		t.Fatalf("expected the registered name, got %q", target.Name)
	}
	if target.Compose.Root != root {
		t.Fatalf("expected the registered path, got %q", target.Compose.Root)
	}
}

// El nombre se compara sin distinguir mayusculas: nadie escribe el nombre exacto.
func TestResolveObserveTargetIgnoresCaseInTheName(t *testing.T) {
	root := newComposeProjectDir(t)
	app := newTestAppContext(t, config.Default())
	seedProject(t, app.DB, root)

	target, err := resolveObserveTarget(context.Background(), app, "REGISTRADO")
	if err != nil {
		t.Fatalf("resolveObserveTarget returned error: %v", err)
	}
	if target.Name != "registrado" {
		t.Fatalf("expected the registered project, got %q", target.Name)
	}
}

// Una ruta a un proyecto que no esta en la base sigue siendo valida: se resuelve
// como proyecto compose suelto, con el nombre del directorio.
func TestResolveObserveTargetAcceptsAnUnregisteredPath(t *testing.T) {
	root := newComposeProjectDir(t)
	app := newTestAppContext(t, config.Default())

	target, err := resolveObserveTarget(context.Background(), app, root)
	if err != nil {
		t.Fatalf("resolveObserveTarget returned error: %v", err)
	}
	if target.Name != filepath.Base(root) {
		t.Fatalf("expected the directory name, got %q", target.Name)
	}
}

func TestResolveObserveTargetExplainsWhatItLookedFor(t *testing.T) {
	app := newTestAppContext(t, config.Default())

	_, err := resolveObserveTarget(context.Background(), app, "no-existe")
	if err == nil {
		t.Fatal("expected an error for an unknown project")
	}
	if !strings.Contains(err.Error(), "registered project") || !strings.Contains(err.Error(), "no-existe") {
		t.Fatalf("the error should name the input and both lookups, got %q", err)
	}
}

func TestObserveNetworkNameFallsBackToTheDefault(t *testing.T) {
	if got := observeNetworkName("  "); got != observe.DefaultNetwork {
		t.Fatalf("expected the default network, got %q", got)
	}
	if got := observeNetworkName("  infra_custom "); got != "infra_custom" {
		t.Fatalf("expected the trimmed name, got %q", got)
	}
}

// El aviso de loopback es lo unico que separa un `observe attach` util de uno que
// deja al contenedor hablandole a si mismo.
func TestWarnObserveLoopbackDSNExplainsTheContainerTrap(t *testing.T) {
	var out bytes.Buffer
	warnObserveLoopbackDSN(&out, "127.0.0.1:9777", observeAddrPlan{})

	text := out.String()
	if !strings.Contains(text, "loopback") || !strings.Contains(text, "container itself") {
		t.Fatalf("expected the loopback explanation:\n%s", text)
	}
	if !strings.Contains(text, "devherd proxy bootstrap") {
		t.Fatalf("expected the way out:\n%s", text)
	}
}

// Un DSN alcanzable pero por una red que el proyecto no toca es el falso positivo
// que ya mordio en campo: tambien tiene que avisar.
func TestWarnObserveLoopbackDSNWarnsOnUnreachableNetwork(t *testing.T) {
	var out bytes.Buffer
	warnObserveLoopbackDSN(&out, "172.18.0.1:9777", observeAddrPlan{
		Project: "demo",
		Reason:  "project demo has no running containers",
		Match:   observe.NetworkInfo{Name: "infra_web"},
	})

	text := out.String()
	if !strings.Contains(text, "no running containers") {
		t.Fatalf("expected the reason to surface:\n%s", text)
	}
	if !strings.Contains(text, "observe status demo") {
		t.Fatalf("expected the verification hint:\n%s", text)
	}
}

func TestWarnObserveLoopbackDSNStaysQuietWhenAllIsWell(t *testing.T) {
	var out bytes.Buffer
	warnObserveLoopbackDSN(&out, "172.18.0.1:9777", observeAddrPlan{Project: "demo"})
	if out.Len() != 0 {
		t.Fatalf("expected no warning for a reachable DSN, got %q", out.String())
	}
}

func TestConfirmDefaultsToYesOnEmptyInput(t *testing.T) {
	cases := map[string]bool{
		"\n":    true,
		"y\n":   true,
		"Y\n":   true,
		"si\n":  true,
		"sí\n":  true,
		"yes\n": true,
		"n\n":   false,
		"no\n":  false,
		"x\n":   false,
	}

	for input, want := range cases {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetIn(strings.NewReader(input))

		if got := confirm(cmd, "¿seguimos?"); got != want {
			t.Fatalf("confirm(%q) = %v, want %v", input, got, want)
		}
	}
}

// Sin entrada (stdin cerrado, por ejemplo en CI) no se asume que si.
func TestConfirmRefusesWithoutInput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(""))

	if confirm(cmd, "¿seguimos?") {
		t.Fatal("expected confirm to refuse when there is no input")
	}
}

func TestPromptDatabaseAcceptsNumbersAndNames(t *testing.T) {
	cases := map[string]string{
		"1\n":        "mysql",
		"mysql\n":    "mysql",
		"2\n":        "mariadb",
		"3\n":        "postgres",
		"postgres\n": "postgres",
		"4\n":        "mongodb",
		"5\n":        "none",
		"":           "none",
	}

	for input, want := range cases {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetIn(strings.NewReader(input))

		if got := promptDatabase(cmd); got != want {
			t.Fatalf("promptDatabase(%q) = %q, want %q", input, got, want)
		}
	}
}

// La sonda de alcanzabilidad es lo que separa un "ok" real de uno medido desde el
// host, que siempre se alcanza a si mismo. El doble de Docker deja probar sus tres
// desenlaces sin daemon.
func TestReportObserveReachabilityReportsSuccess(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	reportObserveReachability(cmd, observeAddrPlan{
		Project: "demo",
		DSN:     "172.18.0.1:9777",
		Match:   observe.NetworkInfo{Name: "infra_web"},
		runner:  &probeRunner{},
	})

	text := out.String()
	if !strings.Contains(text, "ok at http://172.18.0.1:9777") {
		t.Fatalf("expected a successful probe:\n%s", text)
	}
	if !strings.Contains(text, "demo on infra_web") {
		t.Fatalf("the label should name project and network:\n%s", text)
	}
}

func TestReportObserveReachabilityExplainsFailure(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	reportObserveReachability(cmd, observeAddrPlan{
		Project: "demo",
		DSN:     "127.0.0.1:9777",
		Match:   observe.NetworkInfo{Name: "infra_web"},
		runner:  &probeRunner{probeFails: true},
	})

	text := out.String()
	if !strings.Contains(text, "FAILED") {
		t.Fatalf("expected a failure line:\n%s", text)
	}
	if !strings.Contains(text, "loopback") {
		t.Fatalf("a loopback DSN has to be called out as the likely cause:\n%s", text)
	}
	if !strings.Contains(text, "observe firewall --apply") {
		t.Fatalf("expected the remediation hint:\n%s", text)
	}
}

// Sin imagen local no se puede sondear: se salta y se dice como hacerlo a mano.
func TestReportObserveReachabilitySkipsWithoutProbeImage(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	reportObserveReachability(cmd, observeAddrPlan{
		DSN:    "172.18.0.1:9777",
		Match:  observe.NetworkInfo{Name: "infra_web"},
		runner: &probeRunner{noImage: true},
	})

	text := out.String()
	if !strings.Contains(text, "skipped") {
		t.Fatalf("expected the probe to be skipped:\n%s", text)
	}
	if !strings.Contains(text, "run it manually") {
		t.Fatalf("expected the manual command:\n%s", text)
	}
}

func TestReportObserveReachabilitySkipsWithoutNetwork(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	reportObserveReachability(cmd, observeAddrPlan{DSN: "127.0.0.1:9777"})

	if !strings.Contains(out.String(), "no DevHerd network could be resolved") {
		t.Fatalf("expected the default reason:\n%s", out.String())
	}
}

// probeRunner simula las dos llamadas de la sonda: buscar una imagen local y
// correr el contenedor que consulta al collector.
type probeRunner struct {
	noImage    bool
	probeFails bool
}

func (p *probeRunner) Run(_ context.Context, _ string, _ string, args ...string) (string, error) {
	if len(args) >= 2 && args[0] == "image" && args[1] == "inspect" {
		if p.noImage {
			return "", errors.New("Error: No such image")
		}
		return "sha256:abc", nil
	}
	if p.probeFails {
		return "", errors.New("connection refused")
	}

	return "200", nil
}
