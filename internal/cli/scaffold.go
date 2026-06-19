package cli

import (
	"bufio"
	"fmt"
	"slices"
	"strings"

	"github.com/devherd/devherd/internal/scaffold"
	"github.com/spf13/cobra"
)

func newScaffoldCmd() *cobra.Command {
	var (
		dryRun bool
		doUp   bool
		force  bool
		dbKind string
		redis  bool
	)

	cmd := &cobra.Command{
		Use:   "scaffold [path]",
		Short: "Generate a docker-compose and manifest for a project without containers",
		Example: `  # Genera el compose (te pregunta la base de datos)
  devherd scaffold ~/dev/mi-app

  # Sin interacción: Postgres + Redis
  devherd scaffold ~/dev/mi-app --db postgres --redis

  # Previsualiza sin escribir, y genera + levanta
  devherd scaffold ~/dev/mi-app --dry-run
  devherd scaffold ~/dev/mi-app --db mysql --up`,
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
			fmt.Fprintf(out, "detectado: %s (%d servicio(s) de app)\n", plan.Framework, len(plan.Services))

			// Base de datos: por flag, o menú interactivo si no se indicó.
			if dbKind == "" {
				dbKind = promptDatabase(cmd)
			}
			dbKind = strings.ToLower(dbKind)
			if !slices.Contains(scaffold.SupportedDatabases, dbKind) {
				return fmt.Errorf("base de datos no soportada %q (opciones: %s)", dbKind, strings.Join(scaffold.SupportedDatabases, ", "))
			}
			if err := plan.AddDatabase(dbKind); err != nil {
				return err
			}

			if redis {
				plan.AddRedis()
			}

			plan.AssignHostPorts()

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
	cmd.Flags().StringVar(&dbKind, "db", "", "Base de datos: mysql|mariadb|postgres|mongodb|none (si se omite, pregunta)")
	cmd.Flags().BoolVar(&redis, "redis", true, "Incluir Redis (--redis=false para omitirlo)")

	return cmd
}

func promptDatabase(cmd *cobra.Command) string {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "\n¿Qué base de datos quieres? (la app se conecta por red interna, sin exponer puertos al host)")
	fmt.Fprintln(out, "  1) MySQL")
	fmt.Fprintln(out, "  2) MariaDB")
	fmt.Fprintln(out, "  3) PostgreSQL")
	fmt.Fprintln(out, "  4) MongoDB")
	fmt.Fprintln(out, "  5) Ninguna")
	fmt.Fprint(out, "> ")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return "none"
	}

	switch strings.TrimSpace(scanner.Text()) {
	case "1", "mysql":
		return "mysql"
	case "2", "mariadb":
		return "mariadb"
	case "3", "postgres", "postgresql":
		return "postgres"
	case "4", "mongodb", "mongo":
		return "mongodb"
	default:
		return "none"
	}
}
