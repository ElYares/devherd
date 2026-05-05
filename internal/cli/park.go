package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/detector"
	"github.com/spf13/cobra"
)

func newParkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "park [path]",
		Short: "Register a directory for automatic project discovery",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			info, err := os.Stat(targetPath)
			if err != nil {
				return fmt.Errorf("stat path: %w", err)
			}

			if !info.IsDir() {
				return fmt.Errorf("path must be a directory")
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			if err := database.InsertPark(cmd.Context(), app.DB, targetPath); err != nil {
				return err
			}

			projects, err := detector.Discover(targetPath)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "parked: %s\n", targetPath)
			if len(projects) == 0 {
				if err := database.PruneDetectedProjectsUnderPath(cmd.Context(), app.DB, targetPath, nil); err != nil {
					return err
				}
				fmt.Fprintln(out, "detected projects: 0")
				return nil
			}

			keepPaths := make([]string, 0, len(projects))
			for _, project := range projects {
				keepPaths = append(keepPaths, project.Path)
				domain := primaryDomain(project.Name, app.Config.LocalTLD)
				if err := database.UpsertProject(cmd.Context(), app.DB, project, domain); err != nil {
					return err
				}
			}

			if err := database.PruneDetectedProjectsUnderPath(cmd.Context(), app.DB, targetPath, keepPaths); err != nil {
				return err
			}

			fmt.Fprintf(out, "detected projects: %d\n\n", len(projects))
			writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "NAME\tFRAMEWORK\tSTACK\tDOMAIN\tPATH")
			for _, project := range projects {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
					project.Name,
					project.Framework,
					project.Stack,
					primaryDomain(project.Name, app.Config.LocalTLD),
					project.Path,
				)
			}

			return writer.Flush()
		},
	}
}
