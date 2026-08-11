package observe

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/devherd/devherd/internal/runner"
)

// DefaultNetwork es la red Docker donde el proxy externo publica los proyectos.
// Ojo: solo se conecta a ella el servicio que publica el proxy, asi que NO se
// puede asumir que sea la red por la que habla el servicio que reporta.
const DefaultNetwork = "infra_web"

// SharedServicesNetwork es la red de Redis y Mailpit. Los servicios de
// aplicacion suelen estar aqui aunque no esten en la red del proxy, asi que
// ambas cuentan para alcanzar el collector.
const SharedServicesNetwork = "infra_net"

// SharedNetworkNames son las redes administradas por DevHerd por las que un
// proyecto puede hablar con el host, sin duplicados ni vacios.
func SharedNetworkNames(proxyNetwork string) []string {
	names := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, name := range []string{strings.TrimSpace(proxyNetwork), DefaultNetwork, SharedServicesNetwork} {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

// NetworkInfo describe lo que necesitamos de una red Docker para decidir por
// que direccion tienen que hablar los contenedores con el collector.
type NetworkInfo struct {
	Name    string
	Gateway string
	Subnet  string
}

// InspectNetwork resuelve el gateway y la subred IPv4 de una red Docker.
func InspectNetwork(ctx context.Context, r runner.Runner, network string) (NetworkInfo, error) {
	network = strings.TrimSpace(network)
	if network == "" {
		return NetworkInfo{}, fmt.Errorf("docker network is required")
	}
	if r == nil {
		r = runner.Cmd{Timeout: 5 * time.Second}
	}

	const format = "{{range .IPAM.Config}}{{.Gateway}}|{{.Subnet}} {{end}}"
	output, err := r.Run(ctx, "", "docker", "network", "inspect", "--format", format, network)
	if err != nil {
		return NetworkInfo{}, fmt.Errorf("inspect docker network %s: %w", network, err)
	}

	for _, entry := range strings.Fields(output) {
		gateway, subnet, _ := strings.Cut(entry, "|")

		// Solo IPv4: el collector escucha en IPv4 y el DSN se construye con un
		// host:puerto plano, sin corchetes.
		ip := net.ParseIP(strings.TrimSpace(gateway))
		if ip == nil || ip.To4() == nil {
			continue
		}

		return NetworkInfo{
			Name:    network,
			Gateway: ip.String(),
			Subnet:  strings.TrimSpace(subnet),
		}, nil
	}

	return NetworkInfo{}, fmt.Errorf("docker network %s has no IPv4 gateway", network)
}

// InspectNetworks resuelve varias redes, sin repetir y descartando en silencio
// las que no existen: el collector escucha en las que haya, no exige que esten
// todas.
func InspectNetworks(ctx context.Context, r runner.Runner, names []string) []NetworkInfo {
	infos := make([]NetworkInfo, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		info, err := InspectNetwork(ctx, r, name)
		if err != nil {
			continue
		}

		infos = append(infos, info)
	}

	return infos
}

// NetworkCoverage cuenta, para cada red, cuantos contenedores del proyecto
// estan conectados a ella. La union no sirve: en un proyecto tipico solo el
// servicio que publica el proxy esta en la red del proxy, asi que elegir esa red
// produce un DSN que el servicio que reporta no puede alcanzar.
type NetworkCoverage struct {
	Containers int
	Networks   map[string]int
}

// ProjectNetworkCoverage inspecciona los contenedores observados del proyecto.
func ProjectNetworkCoverage(ctx context.Context, r runner.Runner, project string) (NetworkCoverage, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return NetworkCoverage{}, fmt.Errorf("project is required")
	}
	if r == nil {
		r = runner.Cmd{Timeout: 10 * time.Second}
	}

	output, err := r.Run(ctx, "", "docker", "ps", "--filter", "label=devherd.project="+project, "--format", "{{.Names}}")
	if err != nil {
		return NetworkCoverage{}, fmt.Errorf("list containers for project %s: %w", project, err)
	}

	containers := strings.Fields(output)
	if len(containers) == 0 {
		return NetworkCoverage{}, nil
	}

	// Una linea por contenedor, para poder contar cobertura en vez de unir.
	args := append([]string{"inspect", "--format", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}} {{end}}"}, containers...)
	output, err = r.Run(ctx, "", "docker", args...)
	if err != nil {
		return NetworkCoverage{}, fmt.Errorf("inspect containers for project %s: %w", project, err)
	}

	coverage := NetworkCoverage{Networks: map[string]int{}}
	for _, line := range strings.Split(output, "\n") {
		names := strings.Fields(line)
		if len(names) == 0 {
			continue
		}

		coverage.Containers++
		for _, name := range names {
			coverage.Networks[name]++
		}
	}

	return coverage, nil
}

// ObservedNetworks son las redes donde hay contenedores observados por DevHerd,
// que son las que el collector tiene que atender ademas de las compartidas.
func ObservedNetworks(ctx context.Context, r runner.Runner) ([]string, error) {
	if r == nil {
		r = runner.Cmd{Timeout: 10 * time.Second}
	}

	output, err := r.Run(ctx, "", "docker", "ps", "--filter", "label=devherd.observe=true", "--format", "{{.Names}}")
	if err != nil {
		return nil, fmt.Errorf("list observed containers: %w", err)
	}

	containers := strings.Fields(output)
	if len(containers) == 0 {
		return nil, nil
	}

	args := append([]string{"inspect", "--format", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}} {{end}}"}, containers...)
	output, err = r.Run(ctx, "", "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("inspect observed containers: %w", err)
	}

	return uniqueFields(output), nil
}

// SelectProjectNetwork elige la red por la que el proyecto hablara con el
// collector.
//
// Manda la estabilidad sobre la cobertura: entre las redes administradas por
// DevHerd se toma la que mas contenedores cubra, porque su subred sobrevive a un
// `compose down`. La red privada del proyecto suele cubrir mas contenedores,
// pero Docker le asigna otra subred al recrearla y dejaria el DSN inyectado
// apuntando a una direccion que ya no existe. Solo se cae a ella cuando ninguna
// red DevHerd cubre nada, que es el caso de un proyecto totalmente aislado.
func SelectProjectNetwork(coverage NetworkCoverage, preferred []string) (string, int) {
	best := ""
	bestCount := 0
	for _, name := range preferred {
		if count := coverage.Networks[name]; count > bestCount {
			best, bestCount = name, count
		}
	}
	if best != "" {
		return best, bestCount
	}

	// Sin red estable: la de mayor cobertura, con desempate por nombre para que
	// la eleccion no dependa del orden aleatorio del mapa.
	for name, count := range coverage.Networks {
		if count > bestCount || (count == bestCount && best != "" && name < best) {
			best, bestCount = name, count
		}
	}

	return best, bestCount
}

func uniqueFields(output string) []string {
	values := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, name := range strings.Fields(output) {
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		values = append(values, name)
	}

	return values
}

// SplitAddr separa host y puerto tolerando direcciones sin puerto y DSN
// completos (`http://devherd@host:puerto/proyecto`), para poder inspeccionar
// tanto un --addr como el DSN ya construido.
func SplitAddr(addr string) (host, port string) {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://")
	if idx := strings.Index(addr, "/"); idx >= 0 {
		addr = addr[:idx]
	}
	if idx := strings.LastIndex(addr, "@"); idx >= 0 {
		addr = addr[idx+1:]
	}
	if addr == "" {
		return "", ""
	}

	if host, port, err := net.SplitHostPort(addr); err == nil {
		return host, port
	}

	return addr, ""
}

// IsLoopbackAddr indica si addr apunta al loopback: un contenedor que la use se
// estara apuntando a si mismo, no al host donde corre el collector.
func IsLoopbackAddr(addr string) bool {
	host, _ := SplitAddr(addr)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// WithHost reescribe el host de addr conservando su puerto.
func WithHost(addr, host string) string {
	_, port := SplitAddr(addr)
	if port == "" {
		_, port = SplitAddr(DefaultAddr)
	}

	return net.JoinHostPort(host, port)
}
