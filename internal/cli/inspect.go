package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/preflight"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [path]",
		Short: "Inspect a compose project for local infra collisions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			cfg := config.Default()
			app, err := loadAppContext(cmd.Context())
			if err == nil {
				defer app.DB.Close()
				cfg = app.Config
			}

			report, err := preflight.Inspect(cmd.Context(), targetPath, cfg)
			if err != nil {
				return err
			}

			writePreflightReport(cmd.OutOrStdout(), report, true)

			return nil
		},
	}
}

func writePreflightReport(out io.Writer, report preflight.Report, includeOK bool) {
	fmt.Fprintf(out, "Project root: %s\n", report.Project.Root)
	fmt.Fprintln(out, "Findings:")
	for _, finding := range report.Findings {
		if !includeOK && finding.Severity == preflight.SeverityOK {
			continue
		}

		fmt.Fprintf(out, "%-5s %-16s %s\n", strings.ToUpper(string(finding.Severity)), finding.Name, finding.Message)
	}
}
