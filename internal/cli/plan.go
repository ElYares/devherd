package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devherd/devherd/internal/compose"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan [path]",
		Short: "Show the resolved compose stack without starting containers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			project, dockerCommand, err := compose.Plan(targetPath)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Project root: %s\n", project.Root)
			fmt.Fprintf(out, "Resolution: %s\n", describeProjectSource(project))
			fmt.Fprintf(out, "Env file: %s\n", describeEnvFile(project))
			fmt.Fprintln(out, "Compose files:")
			for _, composeFile := range project.ComposeFiles {
				fmt.Fprintf(out, "- %s\n", composeFile)
			}
			fmt.Fprintln(out, "Base command:")
			fmt.Fprintln(out, strings.Join(dockerCommand, " "))
			fmt.Fprintln(out, "Examples:")
			fmt.Fprintln(out, strings.Join(append(append([]string{}, dockerCommand...), "config"), " "))
			fmt.Fprintln(out, strings.Join(append(append([]string{}, dockerCommand...), "up", "--build", "-d"), " "))
			fmt.Fprintln(out, strings.Join(append(append([]string{}, dockerCommand...), "down"), " "))

			return nil
		},
	}
}

func describeProjectSource(project compose.Project) string {
	if project.Source == compose.ProjectSourceManifest {
		return filepath.Join(project.Root, ".devherd.yml")
	}

	return "compose autodetect"
}

func describeEnvFile(project compose.Project) string {
	if project.EnvFile != "" {
		return project.EnvFile
	}

	defaultEnv := filepath.Join(project.Root, ".env")
	if _, err := os.Stat(defaultEnv); err == nil {
		return fmt.Sprintf("compose default (%s)", defaultEnv)
	}

	return "compose default (none detected)"
}
