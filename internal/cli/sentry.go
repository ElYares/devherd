package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSentryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sentry",
		Short: "Manage Sentry integration",
	}

	cmd.AddCommand(
		newSentryInitCmd(),
		newSentrySetDSNCmd(),
		newSentryTestCmd(),
	)

	return cmd
}

func newSentryInitCmd() *cobra.Command {
	var stack string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "init [project]",
		Short: "Initialize Sentry SDK for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			if stack == "" {
				return fmt.Errorf("required flag(s) \"stack\" not set")
			}

			if dryRun {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Sentry dry run")
				fmt.Fprintf(out, "project: %s\n", project)
				fmt.Fprintf(out, "stack: %s\n", stack)
				fmt.Fprintln(out, "provider: sentry-cloud")
				fmt.Fprintln(out, "planned steps:")
				fmt.Fprintln(out, "- detect supported package manager and project layout")
				fmt.Fprintln(out, "- install the official Sentry SDK for the selected stack")
				fmt.Fprintln(out, "- write DSN and environment config into local project files")
				fmt.Fprintln(out, "- prepare a test event command for validation")
				return nil
			}

			return notImplemented("sentry init apply mode")
		},
	}

	cmd.Flags().StringVar(&stack, "stack", "", "Project stack: laravel, node, python or go")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview planned Sentry changes without modifying project files")
	_ = cmd.MarkFlagRequired("stack")

	return cmd
}

func newSentrySetDSNCmd() *cobra.Command {
	var dsn string

	cmd := &cobra.Command{
		Use:   "set-dsn [project]",
		Short: "Set a Sentry DSN for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("sentry set-dsn")
		},
	}

	cmd.Flags().StringVar(&dsn, "dsn", "", "Sentry DSN")
	_ = cmd.MarkFlagRequired("dsn")

	return cmd
}

func newSentryTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [project]",
		Short: "Send a test event to Sentry for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("sentry test")
		},
	}
}
