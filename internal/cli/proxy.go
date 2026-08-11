package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/dns"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

var syncHosts = dns.SyncHosts

// Indireccion para poder sustituirla en pruebas, igual que syncHosts.
var buildExternalProject = proxy.BuildExternalProject

// resolveExternalProjects arma la ruta de proxy de cada proyecto y devuelve,
// aparte, los nombres de los que no se pudieron resolver.
//
// Un proyecto sin compose ni metadata de proxy no puede tener ruta, pero
// tampoco debe dejar sin ruta a los demas: antes, un solo repo mal registrado
// abortaba el apply completo y ningun dominio quedaba publicado.
//
// Cuando 'explicit' es true el usuario nombro un proyecto concreto, y entonces
// no se omite nada: merece saber por que ese no se pudo.
func resolveExternalProjects(cfg config.Config, projects []database.ProjectRecord, explicit bool) ([]proxy.ExternalProject, []string, error) {
	resolved := make([]proxy.ExternalProject, 0, len(projects))
	skipped := make([]string, 0)

	for _, project := range projects {
		externalProject, err := buildExternalProject(cfg, project)
		if err != nil {
			if explicit {
				return nil, nil, err
			}

			slog.Warn("proxy: skipping project; cannot resolve proxy target",
				"project", project.Name,
				"path", project.Path,
				"error", err)
			skipped = append(skipped, project.Name)

			continue
		}

		resolved = append(resolved, externalProject)
	}

	return resolved, skipped, nil
}

func newProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Manage reverse proxy configuration",
	}

	cmd.AddCommand(
		newProxyApplyCmd(),
		newProxyBootstrapCmd(),
	)

	return cmd
}

func newProxyApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply [project]",
		Short: "Render proxy configuration, sync local hosts, and reload Caddy",
		Example: `  # Aplica el proxy para todos los proyectos registrados
  devherd proxy apply

  # Solo para un proyecto
  devherd proxy apply mi-app`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := ""
			if len(args) == 1 {
				projectName = args[0]
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = app.DB.Close() }()

			projects, err := database.ListProjects(cmd.Context(), app.DB)
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				return fmt.Errorf("no registered projects found")
			}

			selectedProjects, err := proxy.SelectProjects(projects, projectName)
			if err != nil {
				return err
			}

			if proxy.UsesDockerExternal(app.Config) {
				externalProjects, skipped, err := resolveExternalProjects(app.Config, selectedProjects, projectName != "")
				if err != nil {
					return err
				}

				for _, externalProject := range externalProjects {
					if _, err := proxy.EnsureComposeOverride(app.Config, externalProject); err != nil {
						return err
					}
					if err := proxy.ConnectProject(cmd.Context(), app.Config, externalProject); err != nil {
						return err
					}
				}

				if len(externalProjects) == 0 {
					return fmt.Errorf("no registered project could be resolved for the external proxy (skipped: %s)", strings.Join(skipped, ", "))
				}

				configPath, domains, err := proxy.ApplyExternalProxy(cmd.Context(), app.Config, externalProjects)
				if err != nil {
					return err
				}

				if err := syncManagedDomains(projects); err != nil {
					return err
				}

				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "caddyfile: %s\n", configPath)
				fmt.Fprintf(out, "domains: %s\n", strings.Join(domains, ", "))
				if len(skipped) > 0 {
					fmt.Fprintf(out, "skipped: %s (sin compose o sin metadata de proxy)\n", strings.Join(skipped, ", "))
				}
				fmt.Fprintln(out, "proxy status: applied")
				return nil
			}

			renderer := proxy.NewRenderer(app.Paths, app.Config)
			renderedConfig, domains, err := renderer.Render(selectedProjects)
			if err != nil {
				return err
			}

			configPath, err := renderer.Write(renderedConfig)
			if err != nil {
				return err
			}

			allDomains := collectDomains(projects)
			if err := syncHosts(allDomains); err != nil {
				return err
			}

			if err := renderer.Apply(configPath); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "caddyfile: %s\n", configPath)
			fmt.Fprintf(out, "domains: %s\n", strings.Join(domains, ", "))
			fmt.Fprintln(out, "proxy status: applied")
			return nil
		},
	}
}

func syncManagedDomains(projects []database.ProjectRecord) error {
	return syncHosts(collectDomains(projects))
}

func collectDomains(projects []database.ProjectRecord) []string {
	domains := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.Domain != "" {
			domains = append(domains, project.Domain)
		}
	}

	return domains
}

func newProxyBootstrapCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Create or refresh the managed external proxy assets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = app.DB.Close() }()

			if !proxy.UsesDockerExternal(app.Config) {
				return fmt.Errorf("proxy bootstrap requires proxy driver %q", proxy.DriverCaddyDockerExternal)
			}

			result, err := proxy.BootstrapExternalProxyWithOptions(app.Config, proxy.BootstrapOptions{
				Force: force,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "external proxy dir: %s\n", result.ExternalDir)
			fmt.Fprintf(out, "external proxy compose: %s\n", result.ComposeFileStatus)
			fmt.Fprintf(out, "external proxy caddyfile: %s\n", result.CaddyfileStatus)
			fmt.Fprintf(out, "external proxy env: %s\n", result.EnvFileStatus)
			fmt.Fprintf(out, "external proxy env example: %s\n", result.EnvExampleStatus)
			fmt.Fprintln(out, "proxy bootstrap: complete")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Rewrite managed compose/Caddyfile templates to match current config")

	return cmd
}
