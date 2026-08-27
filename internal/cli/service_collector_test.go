package cli

import (
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/observe"
	"github.com/devherd/devherd/internal/services"
)

// **La red que importa es la del servicio, no la del proyecto.** El DSN del plan
// elige la red que mas contenedores del proyecto comparten, que es lo correcto
// para un reporter dentro de ese proyecto. Prometheus corre en infra_net.
//
// Medido: el plan devolvia el gateway de infra_web (172.18.0.1) mientras
// Prometheus vivia en infra_net (172.20.0.1).
func TestCollectorAddrUsesTheSharedServicesNetworkGateway(t *testing.T) {
	plan := observeAddrPlan{
		DSN: "172.18.0.1:9777",
		Networks: []observe.NetworkInfo{
			{Name: "infra_web", Gateway: "172.18.0.1"},
			{Name: services.NetworkName, Gateway: "172.20.0.1"},
		},
	}

	addr, warning := collectorAddrFromPlan(plan, func(_, target string) observe.Reachability {
		return observe.Reachability{Address: target, Reachable: true}
	})

	if addr != "172.20.0.1:9777" {
		t.Errorf("expected the %s gateway, got %q", services.NetworkName, addr)
	}
	if warning != "" {
		t.Errorf("a reachable collector needs no warning, got %q", warning)
	}
}

// El caso que aparecio en la prueba manual: el gateway existe y aun asi no se
// llega, porque el cortafuegos del host filtra el trafico de los contenedores.
// Comprobarlo solo por "no es loopback" habria dejado pasar esto en silencio.
func TestCollectorAddrWarnsWhenTheProbeCannotReachIt(t *testing.T) {
	plan := observeAddrPlan{
		DSN:      "172.20.0.1:9777",
		Networks: []observe.NetworkInfo{{Name: services.NetworkName, Gateway: "172.20.0.1", Subnet: "172.20.0.0/16"}},
	}

	addr, warning := collectorAddrFromPlan(plan, func(_, target string) observe.Reachability {
		return observe.Reachability{Address: target, Reachable: false}
	})

	if addr != "172.20.0.1:9777" {
		t.Errorf("the address is still written so the user can fix it, got %q", addr)
	}
	if warning == "" {
		t.Fatal("an unreachable collector must warn before Prometheus starts")
	}
	for _, want := range []string{"did not answer", "target will stay down", "--force"} {
		if !strings.Contains(warning, want) {
			t.Errorf("the warning should mention %q, got:\n%s", want, warning)
		}
	}
}

// Skipped es "no habia imagen local para sondear", no "no se llega". Tratarlo
// como fallo asustaria sin motivo cada vez que falte una imagen de curl.
func TestCollectorAddrDoesNotWarnWhenTheProbeWasSkipped(t *testing.T) {
	plan := observeAddrPlan{
		DSN:      "172.20.0.1:9777",
		Networks: []observe.NetworkInfo{{Name: services.NetworkName, Gateway: "172.20.0.1"}},
	}

	_, warning := collectorAddrFromPlan(plan, func(_, target string) observe.Reachability {
		return observe.Reachability{Address: target, Skipped: true, Reason: "no local image"}
	})

	if warning != "" {
		t.Errorf("a skipped probe is not a failure, got:\n%s", warning)
	}
}

// Sin la red compartida no hay direccion que sirva, y hay que decirlo: 127.0.0.1
// dentro de un contenedor es el propio contenedor.
func TestCollectorAddrWarnsWhenTheSharedNetworkIsMissing(t *testing.T) {
	plan := observeAddrPlan{
		DSN:      "127.0.0.1:9777",
		Networks: []observe.NetworkInfo{{Name: "infra_web", Gateway: "172.18.0.1"}},
		Reason:   "",
	}

	addr, warning := collectorAddrFromPlan(plan, func(_, target string) observe.Reachability {
		t.Fatal("the probe should not run when there is no gateway to probe")

		return observe.Reachability{}
	})

	if addr != "127.0.0.1:9777" {
		t.Errorf("unexpected address %q", addr)
	}
	if !strings.Contains(warning, services.NetworkName) {
		t.Errorf("the warning should name the missing network, got:\n%s", warning)
	}
}
