package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/services"
	"github.com/spf13/cobra"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage shared local development services",
	}

	cmd.AddCommand(
		newServiceActionCmd("start"),
		newServiceActionCmd("stop"),
		newServiceActionCmd("status"),
	)

	return cmd
}

func newServiceActionCmd(action string) *cobra.Command {
	args := cobra.ExactArgs(1)
	if action == "status" {
		args = cobra.MaximumNArgs(1)
	}

	return &cobra.Command{
		Use:   action + " [service]",
		Short: serviceActionShort(action),
		Args:  args,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}

			if err := paths.Ensure(); err != nil {
				return fmt.Errorf("create local directories: %w", err)
			}

			manager := services.NewManager(paths)
			service := ""
			if len(args) == 1 {
				service = args[0]
			}

			output, err := runServiceAction(cmd, manager, action, service)
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}
}

func runServiceAction(cmd *cobra.Command, manager services.Manager, action, service string) (string, error) {
	switch action {
	case "start":
		return manager.Start(cmd.Context(), service)
	case "stop":
		return manager.Stop(cmd.Context(), service)
	case "status":
		return manager.Status(cmd.Context(), service)
	default:
		return "", fmt.Errorf("unsupported service action %q", action)
	}
}

func serviceActionShort(action string) string {
	switch action {
	case "start":
		return "Start a shared service"
	case "stop":
		return "Stop a shared service"
	case "status":
		return "Show status for a shared service"
	default:
		return "Manage a shared service"
	}
}
