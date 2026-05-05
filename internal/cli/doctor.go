package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate local host prerequisites for the MVP",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Default()
			if app, err := loadAppContext(cmd.Context()); err == nil {
				cfg = app.Config
				app.DB.Close()
			}

			report := doctor.RunWithConfig(cmd.Context(), cfg)
			out := cmd.OutOrStdout()

			for _, check := range report.Checks {
				fmt.Fprintf(out, "%-5s %-16s %s\n", statusLabel(check.Status), check.Name, check.Message)
			}

			fmt.Fprintf(out, "\nsummary: %d failure(s), %d warning(s)\n", report.FailureCount(), report.WarningCount())
			if report.HasFailures() {
				return fmt.Errorf("doctor found %d failure(s)", report.FailureCount())
			}

			return nil
		},
	}
}

func statusLabel(status doctor.Status) string {
	switch status {
	case doctor.StatusOK:
		return "OK"
	case doctor.StatusWarn:
		return "WARN"
	case doctor.StatusFail:
		return "FAIL"
	default:
		return "INFO"
	}
}
