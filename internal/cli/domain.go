package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/database"
	"github.com/spf13/cobra"
)

func newDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Manage project domains",
	}

	cmd.AddCommand(newDomainSetCmd())

	return cmd
}

func newDomainSetCmd() *cobra.Command {
	var domain string

	cmd := &cobra.Command{
		Use:   "set [project]",
		Short: "Set the primary domain for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if domain == "" {
				return fmt.Errorf("required flag(s) \"domain\" not set")
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			normalizedDomain, err := normalizeDomain(domain, app.Config.LocalTLD)
			if err != nil {
				return err
			}

			projectName := args[0]
			if err := database.SetPrimaryDomain(cmd.Context(), app.DB, projectName, normalizedDomain); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "project: %s\n", projectName)
			fmt.Fprintf(out, "primary domain: %s\n", normalizedDomain)
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Primary domain or short name for the project")
	_ = cmd.MarkFlagRequired("domain")

	return cmd
}
