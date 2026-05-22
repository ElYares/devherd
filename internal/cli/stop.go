package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [path]",
		Short: "Stop a compose-based project without removing proxy state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				output, fallbackErr := compose.Stop(cmd.Context(), targetPath)
				if output != "" {
					fmt.Fprintln(cmd.OutOrStdout(), output)
				}

				return fallbackErr
			}
			defer app.DB.Close()

			project, err := compose.ResolveProject(targetPath)
			if err != nil {
				return err
			}

			if proxy.UsesDockerExternal(app.Config) {
				overridePath := filepath.Join(project.Root, proxy.ManagedComposeOverrideFile)
				if _, statErr := os.Stat(overridePath); statErr == nil {
					project.ComposeFiles = append(project.ComposeFiles, overridePath)
				}
			}

			output, err := compose.StopProject(cmd.Context(), project)
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}
}
