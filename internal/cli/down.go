package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/compose"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [path]",
		Short: "Stop a compose-based project from the given path or current directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := ""
			if len(args) == 1 {
				targetPath = args[0]
			}

			output, err := compose.Down(cmd.Context(), targetPath)
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return err
		},
	}
}
