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
	Driver    string `json:"driver"`
	HTTPPort  int    `json:"http_port"`
	HTTPSPort int    `json:"https_port"`
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
	return Config{
		SchemaVersion:  1,
		AppName:        "DevHerd Ubuntu",
		LocalTLD:       "test",
		RuntimeManager: "mise",
		Proxy: ProxyConfig{
			Driver:    "caddy",
			HTTPPort:  80,
			HTTPSPort: 443,
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
