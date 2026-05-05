package config

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Proxy.Driver != "caddy" {
		t.Fatalf("expected caddy proxy, got %q", cfg.Proxy.Driver)
	}

	if cfg.LocalTLD != "test" {
		t.Fatalf("expected test TLD, got %q", cfg.LocalTLD)
	}

	if cfg.Observability.Provider != "sentry-cloud" {
		t.Fatalf("expected sentry-cloud provider, got %q", cfg.Observability.Provider)
	}

	if cfg.Proxy.ExternalDir == "" {
		t.Fatalf("expected external proxy dir default to be set")
	}

	if cfg.Proxy.ExternalNetwork != "infra_web" {
		t.Fatalf("expected infra_web external network, got %q", cfg.Proxy.ExternalNetwork)
	}

	if cfg.Proxy.ExternalContainerName != "infra_caddy" {
		t.Fatalf("expected infra_caddy container name, got %q", cfg.Proxy.ExternalContainerName)
	}
}
