package proxy

import (
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/config"
)

// El dominio de un servicio compartido usa el mismo TLD que los proyectos.
// Mezclar `.test` y `.localhost` en la misma maquina es como se acaba probando en
// el host equivocado.
func TestSharedServiceDomainFollowsTheProjectTLD(t *testing.T) {
	cases := []struct {
		tld    string
		driver string
		want   string
	}{
		{tld: "localhost", want: "jupyter.localhost"},
		{tld: "test", want: "jupyter.test"},
		// Sin TLD configurado se cae al del driver.
		{tld: "", driver: DriverCaddyDockerExternal, want: "jupyter.localhost"},
		{tld: "", driver: "caddy", want: "jupyter.test"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			cfg := config.Config{LocalTLD: tc.tld}
			cfg.Proxy.Driver = tc.driver

			if got := SharedServiceDomain(cfg, "jupyter"); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// El nombre del servicio se normaliza: un dominio con mayusculas o espacios no es
// un dominio.
func TestSharedServiceDomainNormalizesTheName(t *testing.T) {
	cfg := config.Config{LocalTLD: "localhost"}

	if got := SharedServiceDomain(cfg, "  Jupyter  "); got != "jupyter.localhost" {
		t.Errorf("expected jupyter.localhost, got %q", got)
	}
}

// Publicar un sitio incompleto no puede escribir un Caddyfile a medias: un
// reverse_proxy sin destino tumba la validacion de Caddy y con ella **todas** las
// rutas, no solo la que falta.
func TestPublishSharedServiceRejectsIncompleteSites(t *testing.T) {
	cases := map[string]SharedServiceSite{
		"sin servicio":   {Container: "infra_jupyter", Port: 8888},
		"sin contenedor": {Service: "jupyter", Port: 8888},
		"sin puerto":     {Service: "jupyter", Container: "infra_jupyter"},
		"puerto cero":    {Service: "jupyter", Container: "infra_jupyter", Port: 0},
	}

	for name, site := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := PublishSharedService(t.Context(), config.Config{}, site); err == nil {
				t.Error("expected an error before touching docker or the Caddyfile")
			}
		})
	}
}

// El bloque de un servicio compartido se renderiza como el de un proyecto: con sus
// marcadores, para poder reemplazarlo despues sin contar llaves.
func TestSharedServiceSiteRendersWithMarkers(t *testing.T) {
	rendered := renderExternalSites([]ExternalProject{{
		Domain: "jupyter.localhost",
		Routes: []Route{{Matcher: "/*", Target: "jupyter:8888"}},
	}})

	for _, want := range []string{
		"# devherd managed start jupyter.localhost",
		"http://jupyter.localhost {",
		"reverse_proxy jupyter:8888",
		"# devherd managed end jupyter.localhost",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("expected the block to contain %q, got:\n%s", want, rendered)
		}
	}
}

// **Publicar un servicio no puede borrar las rutas de los proyectos.** El merge
// solo reemplaza los dominios que se le pasan.
func TestPublishingAServiceKeepsTheProjectRoutes(t *testing.T) {
	existing := `# devherd managed start aang.localhost
http://aang.localhost {
	handle {
		reverse_proxy aang-app:80
	}
}
# devherd managed end aang.localhost
`

	merged := stripManagedDomains(existing, []string{"jupyter.localhost"})
	if !strings.Contains(merged, "aang.localhost") {
		t.Errorf("the project route was dropped:\n%s", merged)
	}
}
