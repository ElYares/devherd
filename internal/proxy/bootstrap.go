package proxy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/devherd/devherd/internal/config"
	proxyexternal "github.com/devherd/devherd/templates/proxy-external"
)

type BootstrapResult struct {
	ExternalDir       string
	ComposeFileStatus string
	CaddyfileStatus   string
	EnvFileStatus     string
	EnvExampleStatus  string
}

type BootstrapOptions struct {
	Force bool
}

func BootstrapExternalProxy(cfg config.Config) (BootstrapResult, error) {
	return BootstrapExternalProxyWithOptions(cfg, BootstrapOptions{})
}

func BootstrapExternalProxyWithOptions(cfg config.Config, options BootstrapOptions) (BootstrapResult, error) {
	return bootstrapExternalProxySettings(externalSettings(cfg), options)
}

func bootstrapExternalProxySettings(settings externalSettingsConfig, options BootstrapOptions) (BootstrapResult, error) {
	if err := os.MkdirAll(settings.Dir, 0o755); err != nil {
		return BootstrapResult{}, fmt.Errorf("create external proxy directory: %w", err)
	}

	result := BootstrapResult{ExternalDir: settings.Dir}

	composeContent, err := renderEmbeddedTemplate(proxyexternal.DockerComposeTemplate, settings)
	if err != nil {
		return BootstrapResult{}, err
	}
	result.ComposeFileStatus, err = ensureManagedFile(
		filepath.Join(settings.Dir, ExternalProxyComposeFile),
		composeContent,
		options.Force,
	)
	if err != nil {
		return BootstrapResult{}, err
	}

	caddyContent, err := renderEmbeddedTemplate(proxyexternal.CaddyfileTemplate, settings)
	if err != nil {
		return BootstrapResult{}, err
	}
	result.CaddyfileStatus, err = ensureManagedFile(
		filepath.Join(settings.Dir, ExternalProxyCaddyfile),
		caddyContent,
		options.Force,
	)
	if err != nil {
		return BootstrapResult{}, err
	}

	envContent, err := renderEmbeddedTemplate(proxyexternal.EnvExampleTemplate, settings)
	if err != nil {
		return BootstrapResult{}, err
	}
	result.EnvFileStatus, err = ensureManagedFile(
		filepath.Join(settings.Dir, ExternalProxyEnvFile),
		envContent,
		false,
	)
	if err != nil {
		return BootstrapResult{}, err
	}
	result.EnvExampleStatus, err = ensureManagedFile(
		filepath.Join(settings.Dir, ".env.example"),
		envContent,
		options.Force,
	)
	if err != nil {
		return BootstrapResult{}, err
	}

	return result, nil
}

func ensureManagedFile(path, content string, force bool) (string, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if string(existing) == content {
			return "reused", nil
		}
		if !force {
			return "reused", nil
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}

		return "updated", nil
	case !os.IsNotExist(err):
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	return "created", nil
}

func renderEmbeddedTemplate(name string, settings externalSettingsConfig) (string, error) {
	payload, err := proxyexternal.Files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(payload))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, settings); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}

	return buffer.String(), nil
}
