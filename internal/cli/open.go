package cli

import (
	"fmt"
	"os/exec"

	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [project]",
		Short: "Open a project in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			projects, err := database.ListProjects(cmd.Context(), app.DB)
			if err != nil {
				return err
			}

			selectedProjects, err := proxy.SelectProjects(projects, args[0])
			if err != nil {
				return err
			}

			url := proxy.URLForDomain(app.Config, selectedProjects[0].Domain)
			if _, err := exec.LookPath("xdg-open"); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), url)
				return nil
			}

			openCmd := exec.Command("xdg-open", url)
			if err := openCmd.Start(); err != nil {
				return fmt.Errorf("open browser: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	}
}
