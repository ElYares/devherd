package observe

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/runner"
)

// probeImages son imagenes con `wget` de busybox que suelen estar ya en local.
// El orden importa: se usa la primera disponible y nunca se descarga ninguna.
var probeImages = []string{"busybox", "alpine", "caddy:2-alpine"}

// ufwConfPath es variable para poder cubrir la deteccion en tests.
var ufwConfPath = "/etc/ufw/ufw.conf"

// Reachability es el resultado de comprobar el collector desde un contenedor.
type Reachability struct {
	Network   string
	Address   string
	Image     string
	Reachable bool
	Skipped   bool
	Reason    string
}

// ProbeFromContainer comprueba si el collector responde *desde dentro* de un
// contenedor conectado a network, que es el unico escenario que importa para un
// DSN inyectado por `observe attach`: probarlo desde el host da falsos positivos
// porque el host alcanza su propio loopback y atraviesa su propio cortafuegos.
//
// No descarga imagenes: si no hay ninguna candidata en local devuelve Skipped
// para que el llamante sugiera el comando equivalente.
func ProbeFromContainer(ctx context.Context, r runner.Runner, network, addr string) Reachability {
	result := Reachability{Network: strings.TrimSpace(network), Address: strings.TrimSpace(addr)}
	if r == nil {
		r = runner.Cmd{Timeout: 30 * time.Second}
	}

	if result.Network == "" || result.Address == "" {
		result.Skipped = true
		result.Reason = "collector address or shared network is unknown"
		return result
	}

	image, ok := firstLocalImage(ctx, r, probeImages)
	if !ok {
		result.Skipped = true
		result.Reason = fmt.Sprintf("no probe image available locally (tried %s)", strings.Join(probeImages, ", "))
		return result
	}
	result.Image = image

	if _, err := r.Run(ctx, "", "docker", probeArgs(result.Network, result.Address, image)...); err != nil {
		result.Reason = err.Error()
		return result
	}

	result.Reachable = true
	return result
}

// ProbeCommand devuelve el comando equivalente a la sonda, para sugerirlo cuando
// no hay imagen local con la que ejecutarla.
func ProbeCommand(network, addr, image string) string {
	if strings.TrimSpace(image) == "" {
		image = probeImages[0]
	}

	return "docker " + strings.Join(probeArgs(network, addr, image), " ")
}

// FirewallHint explica como abrir el paso contenedor -> host. Con ufw activo la
// regla es obligatoria: los puertos publicados por Docker funcionan porque su
// DNAT precede a las cadenas de ufw, pero el collector es un listener normal del
// host y su trafico cae en INPUT.
func FirewallHint(info NetworkInfo, addr string) string {
	rules := FirewallRules([]NetworkInfo{info}, addr)
	if len(rules) == 0 {
		return ""
	}

	if ufwEnabled() {
		return "ufw is enabled on this host, so container traffic needs an explicit rule: " + rules[0].Command()
	}

	return "if a host firewall is filtering container traffic, allow it: " + rules[0].Command()
}

func probeArgs(network, addr, image string) []string {
	return []string{
		"run", "--rm",
		"--network", network,
		"--entrypoint", "wget",
		image,
		"-q", "-T", "3", "-O", "-",
		"http://" + addr + "/health",
	}
}

func firstLocalImage(ctx context.Context, r runner.Runner, candidates []string) (string, bool) {
	for _, image := range candidates {
		if _, err := r.Run(ctx, "", "docker", "image", "inspect", "--format", "{{.Id}}", image); err == nil {
			return image, true
		}
	}

	return "", false
}

// ufwEnabled lee la configuracion de ufw en vez de ejecutar `ufw status`, que
// exige root y romperia el diagnostico sin privilegios.
func ufwEnabled() bool {
	payload, err := os.ReadFile(ufwConfPath)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(payload), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "ENABLED=yes") {
			return true
		}
	}

	return false
}
