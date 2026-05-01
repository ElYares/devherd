package proxy

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
)

const adminAddress = "127.0.0.1:2020"

//go:embed Caddyfile.tmpl
var caddyTemplate string

type Route struct {
	Matcher string
	Target  string
}

type Site struct {
	Domain string
	Routes []Route
}

type Renderer struct {
	paths config.Paths
	cfg   config.Config
}

func NewRenderer(paths config.Paths, cfg config.Config) *Renderer {
	return &Renderer{
		paths: paths,
		cfg:   cfg,
	}
}

func (r *Renderer) Render(projects []database.ProjectRecord) (string, []string, error) {
	if len(projects) == 0 {
		return "", nil, errors.New("no projects available for proxy rendering")
	}

	sites := make([]Site, 0, len(projects))
	domains := make([]string, 0, len(projects))
	for _, project := range projects {
		site, err := r.projectSite(project)
		if err != nil {
			return "", nil, err
		}

		sites = append(sites, site)
		domains = append(domains, project.Domain)
	}

	tmpl, err := template.New("caddyfile").Parse(caddyTemplate)
	if err != nil {
		return "", nil, fmt.Errorf("parse caddy template: %w", err)
	}

	var rendered bytes.Buffer
	data := struct {
		AdminAddress string
		HTTPPort     int
		Sites        []Site
	}{
		AdminAddress: adminAddress,
		HTTPPort:     r.cfg.Proxy.HTTPPort,
		Sites:        sites,
	}

	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", nil, fmt.Errorf("render caddy template: %w", err)
	}

	return rendered.String(), domains, nil
}

func (r *Renderer) Write(content string) (string, error) {
	target := filepath.Join(r.paths.ProxyDir, "Caddyfile")
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write caddyfile: %w", err)
	}

	return target, nil
}

func (r *Renderer) Apply(configPath string) error {
	if _, err := exec.LookPath("caddy"); err != nil {
		return errors.New("caddy is not installed or not available in PATH")
	}

	if err := sudoValidate(); err != nil {
		return err
	}

	if err := runInteractive("sudo", "caddy", "validate", "--config", configPath); err != nil {
		return fmt.Errorf("validate caddy config: %w", err)
	}

	if err := runInteractive("sudo", "caddy", "reload", "--config", configPath, "--address", adminAddress); err == nil {
		return nil
	}

	pidPath := filepath.Join(r.paths.StateDir, "caddy.pid")
	if err := runInteractive("sudo", "caddy", "start", "--config", configPath, "--pidfile", pidPath); err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}

	return nil
}

func (r *Renderer) projectSite(project database.ProjectRecord) (Site, error) {
	switch project.Framework {
	case "vue+flask":
		return Site{
			Domain: project.Domain,
			Routes: []Route{
				{Matcher: "/api/*", Target: "127.0.0.1:8000"},
				{Matcher: "/*", Target: "127.0.0.1:5173"},
			},
		}, nil
	case "flask":
		return Site{
			Domain: project.Domain,
			Routes: []Route{
				{Matcher: "/*", Target: "127.0.0.1:8000"},
			},
		}, nil
	case "vue":
		return Site{
			Domain: project.Domain,
			Routes: []Route{
				{Matcher: "/*", Target: "127.0.0.1:5173"},
			},
		}, nil
	default:
		return Site{}, fmt.Errorf("proxy apply does not support framework %q for project %q yet", project.Framework, project.Name)
	}
}

func sudoValidate() error {
	return runInteractive("sudo", "-v")
}

func runInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func URLForDomain(cfg config.Config, domain string) string {
	if cfg.Proxy.HTTPPort == 80 {
		return "http://" + domain
	}

	return "http://" + domain + ":" + strconv.Itoa(cfg.Proxy.HTTPPort)
}

func SelectProjects(projects []database.ProjectRecord, projectName string) ([]database.ProjectRecord, error) {
	if projectName == "" {
		return projects, nil
	}

	for _, project := range projects {
		if strings.EqualFold(project.Name, projectName) {
			return []database.ProjectRecord{project}, nil
		}
	}

	return nil, fmt.Errorf("project %q not found", projectName)
}
