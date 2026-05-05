package cli

import (
	"fmt"
	"strings"

	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/dns"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

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
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := ""
			if len(args) == 1 {
				projectName = args[0]
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

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
				externalProjects := make([]proxy.ExternalProject, 0, len(selectedProjects))
				for _, project := range selectedProjects {
					externalProject, err := proxy.BuildExternalProject(app.Config, project)
					if err != nil {
						return err
					}

					if _, err := proxy.EnsureComposeOverride(app.Config, externalProject); err != nil {
						return err
					}
					if err := proxy.ConnectProject(cmd.Context(), app.Config, externalProject); err != nil {
						return err
					}

					externalProjects = append(externalProjects, externalProject)
				}

				configPath, domains, err := proxy.ApplyExternalProxy(cmd.Context(), app.Config, externalProjects)
				if err != nil {
					return err
				}

				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "caddyfile: %s\n", configPath)
				fmt.Fprintf(out, "domains: %s\n", strings.Join(domains, ", "))
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
			if err := dns.SyncHosts(allDomains); err != nil {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

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
