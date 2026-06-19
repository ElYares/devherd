package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/version"
	"github.com/spf13/cobra"
)

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	var logOpts logOptions

	cmd := &cobra.Command{
		Use:           "devherd",
		Short:         "Ubuntu-first local development platform",
		Long:          "DevHerd administra proyectos locales, dominios .test, servicios compartidos y bootstrap de Sentry.",
		Version:       version.String(),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			setupLogging(logOpts)
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&logOpts.verbose, "verbose", false, "Enable debug-level diagnostic logging on stderr")
	cmd.PersistentFlags().BoolVar(&logOpts.json, "log-json", false, "Emit diagnostic logs as JSON on stderr")

	cmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")
	cmd.AddCommand(
		newInitCmd(),
		newDoctorCmd(),
		newParkCmd(),
		newListCmd(),
		newDomainCmd(),
		newProxyCmd(),
		newPlanCmd(),
		newInspectCmd(),
		newUpCmd(),
		newServeCmd(),
		newStopCmd(),
		newDownCmd(),
		newOpenCmd(),
		newLogsCmd(),
		newServiceCmd(),
		newObserveCmd(),
		newSentryCmd(),
	)

	return cmd
}

func notImplemented(feature string) error {
	return fmt.Errorf("%s is not implemented yet", feature)
}
