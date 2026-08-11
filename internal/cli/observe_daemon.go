package cli

import (
	"fmt"
	"os"

	"github.com/devherd/devherd/internal/observe"
	"github.com/spf13/cobra"
)

func newObserveDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the collector as a systemd user service",
		Long: "The collector is a foreground process, so closing its terminal stops ingesting and errors are lost silently. " +
			"This installs it as a systemd --user unit that starts on login and restarts on failure.",
	}

	cmd.AddCommand(
		newObserveDaemonInstallCmd(),
		newObserveDaemonUninstallCmd(),
		newObserveDaemonStatusCmd(),
	)

	return cmd
}

func newObserveDaemonInstallCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install and enable the collector user service",
		Example: "  devherd observe daemon install",
		RunE: func(cmd *cobra.Command, _ []string) error {
			binary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve devherd binary: %w", err)
			}

			path, err := observe.InstallDaemon(cmd.Context(), binary, addr)
			if path != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "unit: %s\n", path)
			}
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "binary: %s\n", binary)
			fmt.Fprintln(out, "observe daemon: installed and started")
			fmt.Fprintln(out, "check it with `devherd observe status` or `devherd observe daemon status`")

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "", "Listen address for the service. Empty keeps the default (loopback plus DevHerd network gateways)")

	return cmd
}

func newObserveDaemonUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the collector user service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := observe.UninstallDaemon(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", path)
			return nil
		},
	}
}

func newObserveDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the collector user service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			output, err := observe.DaemonStatus(cmd.Context())
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
				return nil
			}

			return err
		},
	}
}
