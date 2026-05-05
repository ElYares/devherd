package cli

import (
	"testing"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

func TestApplyInitOverridesSetsLocalhostForExternalProxy(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("proxy", proxy.DriverCaddy, "")
	cmd.Flags().String("tld", "test", "")
	if err := cmd.Flags().Set("proxy", proxy.DriverCaddyDockerExternal); err != nil {
		t.Fatalf("set proxy flag: %v", err)
	}

	cfg := config.Default()
	if err := applyInitOverrides(cmd, &cfg, proxy.DriverCaddyDockerExternal, "test", "mise"); err != nil {
		t.Fatalf("applyInitOverrides returned error: %v", err)
	}

	if cfg.Proxy.Driver != proxy.DriverCaddyDockerExternal {
		t.Fatalf("unexpected proxy driver: %q", cfg.Proxy.Driver)
	}

	if cfg.LocalTLD != "localhost" {
		t.Fatalf("expected localhost TLD, got %q", cfg.LocalTLD)
	}

	if cfg.DNS.ManagedSuffix != "localhost" {
		t.Fatalf("expected localhost managed suffix, got %q", cfg.DNS.ManagedSuffix)
	}
}

func TestApplyInitOverridesKeepsExplicitTLD(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("proxy", proxy.DriverCaddy, "")
	cmd.Flags().String("tld", "test", "")
	if err := cmd.Flags().Set("proxy", proxy.DriverCaddyDockerExternal); err != nil {
		t.Fatalf("set proxy flag: %v", err)
	}
	if err := cmd.Flags().Set("tld", "lan"); err != nil {
		t.Fatalf("set tld flag: %v", err)
	}

	cfg := config.Default()
	if err := applyInitOverrides(cmd, &cfg, proxy.DriverCaddyDockerExternal, "lan", "mise"); err != nil {
		t.Fatalf("applyInitOverrides returned error: %v", err)
	}

	if cfg.LocalTLD != "lan" {
		t.Fatalf("expected explicit TLD to win, got %q", cfg.LocalTLD)
	}
}
