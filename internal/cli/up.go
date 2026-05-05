package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/compose"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [path]",
		Short: "Start a compose-based project from the given path or current directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				output, fallbackErr := compose.Up(cmd.Context(), targetPath)
				if output != "" {
					fmt.Fprintln(cmd.OutOrStdout(), output)
				}

				return fallbackErr
			}
			defer app.DB.Close()

			project, err := prepareComposeProject(cmd.Context(), app, targetPath)
			if err != nil {
				return err
			}

			output, err := compose.UpProject(cmd.Context(), project)
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}
}
