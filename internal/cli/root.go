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
	cmd := &cobra.Command{
		Use:           "devherd",
		Short:         "Local development platform for Docker-based projects",
		Long:          "DevHerd administra proyectos locales, dominios .test/.localhost, servicios compartidos y observabilidad local.",
		Version:       version.String(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

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
