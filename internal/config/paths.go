package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigDir  string
	ConfigFile string
	DataDir    string
	DBFile     string
	StateDir   string
	LogsDir    string
	ProxyDir   string
	ComposeDir string
}

func ResolvePaths() (Paths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	dataRoot := envOrDefault("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	stateRoot := envOrDefault("XDG_STATE_HOME", filepath.Join(homeDir, ".local", "state"))

	configDir := filepath.Join(configRoot, "devherd")
	dataDir := filepath.Join(dataRoot, "devherd")
	stateDir := filepath.Join(stateRoot, "devherd")

	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.json"),
		DataDir:    dataDir,
		DBFile:     filepath.Join(dataDir, "devherd.db"),
		StateDir:   stateDir,
		LogsDir:    filepath.Join(stateDir, "logs"),
		ProxyDir:   filepath.Join(dataDir, "proxy"),
		ComposeDir: filepath.Join(dataDir, "compose"),
	}, nil
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.ConfigDir, p.DataDir, p.StateDir, p.LogsDir, p.ProxyDir, p.ComposeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
