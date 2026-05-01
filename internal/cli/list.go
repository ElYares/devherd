package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/devherd/devherd/internal/database"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered projects",
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

			out := cmd.OutOrStdout()
			if asJSON {
				payload, err := json.MarshalIndent(projects, "", "  ")
				if err != nil {
					return fmt.Errorf("encode projects: %w", err)
				}

				fmt.Fprintln(out, string(payload))
				return nil
			}

			if len(projects) == 0 {
				fmt.Fprintln(out, "no projects registered")
				return nil
			}

			writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "NAME\tFRAMEWORK\tSTACK\tDOMAIN\tSTATUS\tPATH")
			for _, project := range projects {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
					project.Name,
					project.Framework,
					project.Stack,
					project.Domain,
					project.Status,
					project.Path,
				)
			}

			return writer.Flush()
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output registered projects as JSON")

	return cmd
}
