package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/preflight"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var force bool
	var noInspect bool

	cmd := &cobra.Command{
		Use:   "up [path]",
		Short: "Start a compose-based project from the given path or current directory",
		Example: `  # Levantar el proyecto del directorio actual
  devherd up

  # Saltar el preflight de colisiones
  devherd up ~/dev/mi-app --no-inspect`,
		Args: cobra.MaximumNArgs(1),
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

			if !noInspect {
				if err := runUpPreflight(cmd, targetPath, app, force); err != nil {
					return err
				}
			}

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

	cmd.Flags().BoolVar(&force, "force", false, "Continue even when preflight detects failures")
	cmd.Flags().BoolVar(&noInspect, "no-inspect", false, "Skip preflight inspection before starting")

	return cmd
}

func runUpPreflight(cmd *cobra.Command, targetPath string, app *appContext, force bool) error {
	report, err := preflight.Inspect(cmd.Context(), targetPath, app.Config)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	switch {
	case report.HasFailures():
		if force {
			fmt.Fprintln(out, "preflight: failures found; continuing because --force was set")
			writePreflightReport(out, report, false)
			fmt.Fprintln(out)
			return nil
		}

		fmt.Fprintln(out, "preflight: failures found")
		writePreflightReport(out, report, false)
		return fmt.Errorf("preflight failed; use --force to continue or --no-inspect to skip")
	case report.HasWarnings():
		fmt.Fprintln(out, "preflight: warnings found")
		writePreflightReport(out, report, false)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "continuing...")
	default:
		fmt.Fprintln(out, "preflight: ok")
	}

	return nil
}
