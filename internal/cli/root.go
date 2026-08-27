package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devherd/devherd/internal/version"
	"github.com/spf13/cobra"
)

func Execute() error {
	// El contexto se cancela con Ctrl-C o SIGTERM. Sin esto, `observe start` moria
	// de golpe: el apagado ordenado del collector —cerrar el servidor, drenar el
	// poller, esperar al WaitGroup— nunca llegaba a correr, porque nada cancelaba
	// su contexto. Estaba escrito y era codigo muerto.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	var logOpts logOptions

	cmd := &cobra.Command{
		Use:           "devherd",
		Short:         "Ubuntu-first local development platform",
		Long:          "DevHerd administra proyectos locales, dominios .test, servicios compartidos y bootstrap de Sentry.",
		Version:       version.Long(),
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
		newScaffoldCmd(),
		newUpCmd(),
		newServeCmd(),
		newStopCmd(),
		newDownCmd(),
		newOpenCmd(),
		newLogsCmd(),
		newServiceCmd(),
		newObserveCmd(),
		newCoverageCmd(),
		newSentryCmd(),
	)

	return cmd
}

func notImplemented(feature string) error {
	return fmt.Errorf("%s is not implemented yet", feature)
}
