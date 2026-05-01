package cli

import "github.com/spf13/cobra"

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
			return notImplemented("service " + action)
		},
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
