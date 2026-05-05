package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	SchemaVersion  int                 `json:"schema_version"`
	AppName        string              `json:"app_name"`
	LocalTLD       string              `json:"local_tld"`
	RuntimeManager string              `json:"runtime_manager"`
	Proxy          ProxyConfig         `json:"proxy"`
	DNS            DNSConfig           `json:"dns"`
	Observability  ObservabilityConfig `json:"observability"`
}

type ProxyConfig struct {
	Driver                string `json:"driver"`
	HTTPPort              int    `json:"http_port"`
	HTTPSPort             int    `json:"https_port"`
	ExternalDir           string `json:"external_dir"`
	ExternalNetwork       string `json:"external_network"`
	ExternalContainerName string `json:"external_container_name"`
}

type DNSConfig struct {
	Driver        string `json:"driver"`
	ResolverIP    string `json:"resolver_ip"`
	ManagedSuffix string `json:"managed_suffix"`
}

type ObservabilityConfig struct {
	Provider string `json:"provider"`
}

func Default() Config {
	externalDir := defaultExternalProxyDir()

	return Config{
		SchemaVersion:  1,
		AppName:        "DevHerd Ubuntu",
		LocalTLD:       "test",
		RuntimeManager: "mise",
		Proxy: ProxyConfig{
			Driver:                "caddy",
			HTTPPort:              80,
			HTTPSPort:             443,
			ExternalDir:           externalDir,
			ExternalNetwork:       "infra_web",
			ExternalContainerName: "infra_caddy",
		},
		DNS: DNSConfig{
			Driver:        "dnsmasq",
			ResolverIP:    "127.0.0.1",
			ManagedSuffix: "test",
		},
		Observability: ObservabilityConfig{
			Provider: "sentry-cloud",
		},
	}
}

func (c *Config) ApplyPathDefaults(paths Paths) {
	if c.Proxy.ExternalDir == "" {
		c.Proxy.ExternalDir = filepath.Join(paths.DataDir, "local_proxy")
	}

	if c.Proxy.ExternalNetwork == "" {
		c.Proxy.ExternalNetwork = "infra_web"
	}

	if c.Proxy.ExternalContainerName == "" {
		c.Proxy.ExternalContainerName = "infra_caddy"
	}

	if c.DNS.ManagedSuffix == "" && c.LocalTLD != "" {
		c.DNS.ManagedSuffix = c.LocalTLD
	}
}

func defaultExternalProxyDir() string {
	paths, err := ResolvePaths()
	if err == nil {
		return filepath.Join(paths.DataDir, "local_proxy")
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil || homeDir == "" {
		return filepath.Join(".local", "share", "devherd", "local_proxy")
	}

	return filepath.Join(homeDir, ".local", "share", "devherd", "local_proxy")
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Config, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tempFile := s.path + ".tmp"
	if err := os.WriteFile(tempFile, payload, 0o600); err != nil {
		return err
	}

	return os.Rename(tempFile, s.path)
}
