package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [path]",
		Short: "Stop a compose-based project from the given path or current directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				output, fallbackErr := compose.Down(cmd.Context(), targetPath)
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

			var externalProject proxy.ExternalProject
			if proxy.UsesDockerExternal(app.Config) {
				externalProject, _ = resolveExternalProject(cmd.Context(), app, project.Root)
			}

			if proxy.UsesDockerExternal(app.Config) {
				overridePath := filepath.Join(project.Root, proxy.ManagedComposeOverrideFile)
				if _, statErr := os.Stat(overridePath); statErr == nil {
					project.ComposeFiles = append(project.ComposeFiles, overridePath)
				}
			}

			output, err := compose.DownProject(cmd.Context(), project)
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			if proxy.UsesDockerExternal(app.Config) {
				overridePath := filepath.Join(project.Root, proxy.ManagedComposeOverrideFile)
				if err := os.Remove(overridePath); err != nil && !os.IsNotExist(err) {
					return err
				}

				if externalProject.Domain != "" {
					if _, err := proxy.RemoveExternalProxy(cmd.Context(), []string{externalProject.Domain}); err != nil {
						return err
					}
				}
			}

			return err
		},
	}
}
