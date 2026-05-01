package cli

import "github.com/spf13/cobra"

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [project]",
		Short: "Tail logs for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("logs")
		},
	}
}
