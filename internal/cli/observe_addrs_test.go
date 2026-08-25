package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// routedRunner responde segun *que* comando se le pide, no segun el orden en que
// llega. El doble secuencial de internal/observe no sirve aqui: planObserveAddrs
// encadena entre tres y seis llamadas a Docker cuyo orden depende de la
// cobertura, y una cola de respuestas se desalinearia al primer cambio.
type routedRunner struct {
	// networks mapea red -> salida de `docker network inspect`, "gateway|subred".
	networks map[string]string
	// containersByLabel mapea el valor de --filter label=... -> nombres.
	containersByLabel map[string]string
	// networksByContainer mapea contenedor -> redes separadas por espacio.
	networksByContainer map[string]string
	calls               [][]string
}

func (r *routedRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))

	switch {
	case len(args) >= 2 && args[0] == "network" && args[1] == "inspect":
		network := args[len(args)-1]
		output, ok := r.networks[network]
		if !ok {
			return "", errors.New("Error: No such network: " + network)
		}

		return output, nil

	case len(args) >= 1 && args[0] == "ps":
		label := ""
		for i, arg := range args {
			if arg == "--filter" && i+1 < len(args) {
				label = strings.TrimPrefix(args[i+1], "label=")
			}
		}

		return r.containersByLabel[label], nil

	case len(args) >= 3 && args[0] == "inspect":
		// Una linea por contenedor: asi es como se cuenta cobertura en vez de unir.
		lines := make([]string, 0, len(args)-3)
		for _, container := range args[3:] {
			lines = append(lines, r.networksByContainer[container])
		}

		return strings.Join(lines, "\n"), nil
	}

	return "", errors.New("unexpected docker call: " + strings.Join(args, " "))
}

// demoRunner arma el caso tipico: un proyecto de tres contenedores cuya red
// privada los cubre a todos, y del que solo el que publica el proxy esta en la
// red de DevHerd.
func demoRunner() *routedRunner {
	return &routedRunner{
		networks: map[string]string{
			"infra_web":    "172.18.0.1|172.18.0.0/16",
			"infra_net":    "172.19.0.1|172.19.0.0/16",
			"demo_default": "172.30.0.1|172.30.0.0/16",
		},
		containersByLabel: map[string]string{
			"devherd.observe=true":  "demo_web_1 demo_db_1 demo_queue_1",
			"devherd.project=demo":  "demo_web_1 demo_db_1 demo_queue_1",
			"devherd.project=solo":  "solo_app_1",
			"devherd.project=vacio": "",
		},
		networksByContainer: map[string]string{
			"demo_web_1":   "infra_web demo_default",
			"demo_db_1":    "demo_default",
			"demo_queue_1": "demo_default",
			"solo_app_1":   "solo_default",
		},
	}
}

// La trampa que ya produjo un falso positivo en campo: la red que mas cubre no
// es la que se elige, porque su subred no sobrevive a un `compose down`.
func TestPlanObserveAddrsPrefersStableNetworkOverCoverage(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "demo",
		Runner:       demoRunner(),
	})

	if plan.Match.Name != "infra_web" {
		t.Fatalf("expected the stable DevHerd network, got %q", plan.Match.Name)
	}
	if plan.Coverage != "1/3" {
		t.Fatalf("expected coverage 1/3 on infra_web, got %q", plan.Coverage)
	}
	if plan.DSN != "172.18.0.1:9777" {
		t.Fatalf("expected the DSN on the infra_web gateway, got %q", plan.DSN)
	}
	if plan.Reason != "" {
		t.Fatalf("expected no warning reason, got %q", plan.Reason)
	}
}

// Un proyecto aislado es el unico caso en que se acepta la subred inestable.
func TestPlanObserveAddrsFallsBackToProjectNetworkWhenIsolated(t *testing.T) {
	runner := demoRunner()
	runner.networks["solo_default"] = "172.31.0.1|172.31.0.0/16"

	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "solo",
		Runner:       runner,
	})

	if plan.Match.Name != "solo_default" {
		t.Fatalf("expected the project network as last resort, got %q", plan.Match.Name)
	}
	if plan.DSN != "172.31.0.1:9777" {
		t.Fatalf("expected the DSN on the project network gateway, got %q", plan.DSN)
	}
}

// El collector escucha en el gateway de *todas* las redes, no solo en la elegida:
// un contenedor de otra red tiene que poder reportar igual.
func TestPlanObserveAddrsBindsEveryResolvedNetwork(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "demo",
		Runner:       demoRunner(),
	})

	want := []string{"127.0.0.1:9777", "172.18.0.1:9777", "172.19.0.1:9777", "172.30.0.1:9777"}
	if len(plan.Bind) != len(want) {
		t.Fatalf("expected %d bind addresses, got %d: %v", len(want), len(plan.Bind), plan.Bind)
	}
	for i, addr := range want {
		if plan.Bind[i] != addr {
			t.Fatalf("bind[%d]: expected %q, got %q", i, addr, plan.Bind[i])
		}
	}
}

// Sin contenedores corriendo no se puede saber la red del proyecto. Tiene que
// decirlo, no elegir a ciegas ni fallar.
func TestPlanObserveAddrsExplainsProjectWithoutContainers(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "vacio",
		Runner:       demoRunner(),
	})

	if plan.Reason == "" {
		t.Fatal("expected a reason explaining that the project has no containers")
	}
	if !strings.Contains(plan.Reason, "vacio") || !strings.Contains(plan.Reason, "devherd up") {
		t.Fatalf("the reason should name the project and how to fix it, got %q", plan.Reason)
	}
	if plan.Match.Name != "infra_web" {
		t.Fatalf("expected the fallback to the first stable network, got %q", plan.Match.Name)
	}
	if plan.Coverage != "" {
		t.Fatalf("expected no coverage when there are no containers, got %q", plan.Coverage)
	}
}

// Con --addr explicito manda el usuario. Las redes se siguen resolviendo, porque
// de ahi salen los avisos y las reglas de cortafuegos.
func TestPlanObserveAddrsRespectsExplicitAddr(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "demo",
		Addr:         "0.0.0.0:9999",
		Explicit:     true,
		Runner:       demoRunner(),
	})

	if len(plan.Bind) != 1 || plan.Bind[0] != "0.0.0.0:9999" {
		t.Fatalf("expected only the explicit bind address, got %v", plan.Bind)
	}
	if plan.DSN != "0.0.0.0:9999" {
		t.Fatalf("expected the DSN untouched, got %q", plan.DSN)
	}
	if len(plan.Networks) == 0 {
		t.Fatal("networks should still be resolved so the command can warn")
	}
}

// Sin daemon de Docker no hay red que resolver, y el plan tiene que explicarlo en
// vez de devolver un DSN que no lleva a ninguna parte.
func TestPlanObserveAddrsReportsWhenNoNetworkResolves(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "demo",
		Runner:       &routedRunner{},
	})

	if !strings.Contains(plan.Reason, "Docker") {
		t.Fatalf("expected a reason pointing at Docker, got %q", plan.Reason)
	}
	if plan.DSN != "127.0.0.1:9777" {
		t.Fatalf("expected the loopback default, got %q", plan.DSN)
	}
}

func TestObserveAddrPlanNetworkNamesListsEveryNetwork(t *testing.T) {
	plan := planObserveAddrs(context.Background(), observeAddrOptions{
		ProxyNetwork: "infra_web",
		Project:      "demo",
		Runner:       demoRunner(),
	})

	names := plan.networkNames()
	for _, want := range []string{"infra_web", "infra_net", "demo_default"} {
		if !strings.Contains(names, want) {
			t.Fatalf("expected %q in %q", want, names)
		}
	}
}
