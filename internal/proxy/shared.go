package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/devherd/devherd/internal/config"
)

// SharedServiceSite publica un servicio compartido en un dominio local, para no
// tener que recordar en que puerto escucha cada uno.
//
// Es la misma maquinaria que usan los proyectos —bloques marcados en el Caddyfile
// administrado— con dos diferencias: el contenedor no sale del registro de
// proyectos, y hay que conectarlo a la red del proxy porque los servicios
// compartidos viven en la suya.
type SharedServiceSite struct {
	// Service es el nombre del servicio compartido, y tambien su alias en la red
	// del proxy: `jupyter` se convierte en `jupyter.localhost`.
	Service string
	// Container es el nombre real del contenedor, que es lo que Docker conecta.
	Container string
	// Port es donde escucha dentro de la red, no el publicado en el host.
	Port int
}

// SharedServiceDomain arma el dominio de un servicio compartido con el mismo TLD
// que usan los proyectos: mezclar `.test` y `.localhost` en la misma maquina es
// como se acaba probando en el host equivocado.
func SharedServiceDomain(cfg config.Config, service string) string {
	tld := strings.TrimSpace(cfg.LocalTLD)
	if tld == "" {
		tld = DefaultTLDForDriver(cfg.Proxy.Driver)
	}

	return strings.ToLower(strings.TrimSpace(service)) + "." + tld
}

// PublishSharedService conecta el contenedor a la red del proxy y le escribe su
// bloque en el Caddyfile administrado. Devuelve el dominio publicado.
//
// Solo toca su propio bloque: `mergeExternalProxyConfig` reemplaza los dominios
// que se le pasan y conserva los demas, asi que publicar un servicio no borra las
// rutas de ningun proyecto.
func PublishSharedService(ctx context.Context, cfg config.Config, site SharedServiceSite) (string, error) {
	if strings.TrimSpace(site.Service) == "" || strings.TrimSpace(site.Container) == "" {
		return "", fmt.Errorf("a shared service site needs a service and a container name")
	}
	if site.Port <= 0 {
		return "", fmt.Errorf("a shared service site needs the port it listens on")
	}

	settings := externalSettings(cfg)
	if err := ensureExternalProxyNetwork(ctx, settings); err != nil {
		return "", err
	}

	// El contenedor vive en la red de los servicios compartidos, donde el proxy no
	// llega. Se conecta tambien a la suya con un alias, que es como Caddy lo nombra
	// en el reverse_proxy. Es el mismo camino que recorre ConnectProject.
	if _, err := runCmd(ctx, "", "docker", "network", "connect",
		"--alias", site.Service, settings.Network, site.Container); err != nil {
		message := err.Error()
		if !strings.Contains(message, "already exists") && !strings.Contains(message, "already connected") {
			return "", fmt.Errorf("connect %s to %s: %w", site.Container, settings.Network, err)
		}
	}

	domain := SharedServiceDomain(cfg, site.Service)
	target := fmt.Sprintf("%s:%d", site.Service, site.Port)

	if _, _, err := ApplyExternalProxy(ctx, cfg, []ExternalProject{{
		Domain: domain,
		Routes: []Route{{Matcher: "/*", Target: target}},
	}}); err != nil {
		return "", err
	}

	return domain, nil
}
