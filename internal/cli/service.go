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

	var force bool

	cmd := &cobra.Command{
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

			output, files, err := runServiceAction(cmd, manager, action, service, force)
			// El aviso de configuracion va por stderr y **antes** de la salida de
			// docker: escribir dentro del directorio de alguien sin decirlo es como
			// se pierde una tarde buscando por que un servicio no toma sus ajustes.
			if notice := services.DescribeFileResults(files); notice != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), notice)
			}
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}

	if action == "start" {
		cmd.Flags().BoolVar(&force, "force", false,
			"Restore managed configuration files from the DevHerd template, keeping a .bak copy")
	}

	return cmd
}

func runServiceAction(
	cmd *cobra.Command,
	manager services.Manager,
	action, service string,
	force bool,
) (string, []services.FileResult, error) {
	switch action {
	case "start":
		return manager.Start(cmd.Context(), service, force)
	case "stop":
		output, err := manager.Stop(cmd.Context(), service)

		return output, nil, err
	case "status":
		output, err := manager.Status(cmd.Context(), service)

		return output, nil, err
	default:
		return "", nil, fmt.Errorf("unsupported service action %q", action)
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
