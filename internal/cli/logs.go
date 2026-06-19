package cli

import (
	"os"
	"path/filepath"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/proxy"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		follow bool
		tail   string
	)

	cmd := &cobra.Command{
		Use:   "logs [path]",
		Short: "Tail logs for a project",
		Example: `  # Últimas 100 líneas
  devherd logs ~/dev/mi-app --tail 100

  # Seguir en vivo
  devherd logs ~/dev/mi-app -f`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			project, err := compose.ResolveProject(targetPath)
			if err != nil {
				return err
			}

			// Alinea los compose files con los que se usaron en `up` (override de
			// proxy externo + observe) para que los logs cubran todos los servicios
			// en ejecución. El app context es opcional: sin él, se usa el proyecto base.
			if app, err := loadAppContext(cmd.Context()); err == nil {
				defer app.DB.Close()

				if proxy.UsesDockerExternal(app.Config) {
					overridePath := filepath.Join(project.Root, proxy.ManagedComposeOverrideFile)
					if _, statErr := os.Stat(overridePath); statErr == nil {
						project.ComposeFiles = append(project.ComposeFiles, overridePath)
					}
				}
				project = appendObserveOverride(project)
			}

			opts := compose.LogsOptions{
				Follow: follow,
				Tail:   tail,
			}

			return compose.LogsProject(cmd.Context(), project, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (stream live)")
	cmd.Flags().StringVar(&tail, "tail", "", "Number of lines to show from the end of the logs (e.g. 100 or all)")

	return cmd
}
