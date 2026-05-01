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
}
