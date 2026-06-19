package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/scaffold"
	"github.com/spf13/cobra"
)

func newScaffoldCmd() *cobra.Command {
	var (
		dryRun bool
		doUp   bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "scaffold [path]",
		Short: "Generate a docker-compose and manifest for a project without containers",
		Example: `  # Genera el compose para el proyecto del directorio actual
  devherd scaffold

  # Previsualiza sin escribir nada
  devherd scaffold ~/dev/mi-app --dry-run

  # Genera y levanta
  devherd scaffold ~/dev/mi-app --up`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			plan, err := scaffold.Detect(targetPath)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "detectado: %s (%d servicios)\n", plan.Framework, len(plan.Services))

			if dryRun {
				fmt.Fprintf(out, "\n--- %s ---\n%s", scaffold.ManagedComposeFile, scaffold.RenderCompose(plan))
				fmt.Fprintf(out, "\n--- %s ---\n%s", scaffold.ManifestFile, scaffold.RenderManifest(plan))
				return nil
			}

			states, err := scaffold.Write(plan, force)
			if err != nil {
				return err
			}
			for _, name := range []string{scaffold.ManagedComposeFile, scaffold.ManifestFile} {
				fmt.Fprintf(out, "  %s: %s\n", name, states[name])
			}

			if doUp {
				upArgs := []string{"up"}
				if targetPath != "" {
					upArgs = append(upArgs, targetPath)
				}
				return runSiblingCommand(cmd, upArgs)
			}

			fmt.Fprintln(out, "\nlisto. Ejecuta `devherd up` para levantarlo.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Muestra el compose y el manifiesto sin escribir archivos")
	cmd.Flags().BoolVar(&doUp, "up", false, "Genera y luego levanta el proyecto (devherd up)")
	cmd.Flags().BoolVar(&force, "force", false, "Sobrescribe archivos generados existentes")

	return cmd
}
