package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devherd/devherd/internal/database"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve [path]",
		Short: "Start a project, apply the proxy and open it (up + proxy apply + open)",
		Example: `  # Levanta el proyecto del directorio actual, aplica el proxy y lo abre
  devherd serve

  # Por ruta explícita
  devherd serve ~/dev/mi-app`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			upArgs := []string{"up"}
			if targetPath != "" {
				upArgs = append(upArgs, targetPath)
			}
			if err := runSiblingCommand(cmd, upArgs); err != nil {
				return err
			}

			if err := runSiblingCommand(cmd, []string{"proxy", "apply"}); err != nil {
				return err
			}

			name, err := projectNameForPath(cmd, targetPath)
			if err != nil || name == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "serve: proyecto levantado y proxy aplicado. No se pudo resolver el proyecto para abrirlo; usa `devherd open <proyecto>`.")
				return nil
			}

			if err := runSiblingCommand(cmd, []string{"open", name}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "serve: no se pudo abrir el proyecto %q: %v\n", name, err)
			}

			return nil
		},
	}
}

// runSiblingCommand ejecuta otro comando del mismo árbol reutilizando su lógica
// exacta (en vez de duplicarla), propagando contexto y writers del padre.
func runSiblingCommand(parent *cobra.Command, argv []string) error {
	target, remaining, err := parent.Root().Find(argv)
	if err != nil {
		return err
	}
	if target.RunE == nil && target.Run == nil {
		return fmt.Errorf("command %q is not runnable", argv[0])
	}

	target.SetContext(parent.Context())
	target.SetOut(parent.OutOrStdout())
	target.SetErr(parent.ErrOrStderr())

	if err := target.ParseFlags(remaining); err != nil {
		return err
	}

	posArgs := target.Flags().Args()
	if err := target.ValidateArgs(posArgs); err != nil {
		return err
	}

	if target.RunE != nil {
		return target.RunE(target, posArgs)
	}
	target.Run(target, posArgs)
	return nil
}

func projectNameForPath(cmd *cobra.Command, targetPath string) (string, error) {
	app, err := loadAppContext(cmd.Context())
	if err != nil {
		return "", err
	}
	defer app.DB.Close()

	abs := targetPath
	if abs == "" {
		if wd, err := os.Getwd(); err == nil {
			abs = wd
		}
	}
	if resolved, err := filepath.Abs(abs); err == nil {
		abs = resolved
	}

	record, found, err := database.FindProjectByPath(cmd.Context(), app.DB, abs)
	if err != nil || !found {
		return "", err
	}

	return record.Name, nil
}
