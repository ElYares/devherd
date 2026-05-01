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

	cmd.AddCommand(newProxyApplyCmd())

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
