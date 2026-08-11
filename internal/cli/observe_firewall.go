package cli

import (
	"fmt"

	"github.com/devherd/devherd/internal/observe"
	"github.com/spf13/cobra"
)

func newObserveFirewallCmd() *cobra.Command {
	var addr string
	var apply bool

	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Show or apply the firewall rules the collector needs",
		Long: "Container to host traffic is filtered by the host firewall: Docker published ports work because their DNAT rules precede it, " +
			"but the collector is a plain host listener. This prints one rule per DevHerd network, since each has its own source subnet and gateway.",
		Example: "  devherd observe firewall\n  devherd observe firewall --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
				ProxyNetwork: observeSharedNetwork(cmd.Context()),
				Addr:         addr,
				Explicit:     cmd.Flags().Changed("addr"),
			})

			out := cmd.OutOrStdout()
			rules := observe.FirewallRules(plan.Networks, plan.DSN)
			if len(rules) == 0 {
				fmt.Fprintln(out, "no DevHerd network is available, so no rule can be derived")
				if plan.Reason != "" {
					fmt.Fprintf(out, "reason: %s\n", plan.Reason)
				}
				return nil
			}

			switch {
			case observe.UFWEnabled():
				fmt.Fprintln(out, "ufw: enabled (rules below are required)")
			case observe.UFWAvailable():
				fmt.Fprintln(out, "ufw: installed but not enabled")
			default:
				fmt.Fprintln(out, "ufw: not installed; translate the rules to your firewall")
			}

			for _, rule := range rules {
				fmt.Fprintf(out, "%s\n", rule.Command())
			}

			if !apply {
				fmt.Fprintln(out, "\nrun with --apply to add them (ufw allow is idempotent)")
				return nil
			}

			if err := observe.ApplyFirewallRules(cmd.Context(), rules); err != nil {
				return err
			}

			fmt.Fprintln(out, "observe firewall: rules applied")
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address the rules should allow")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the rules with sudo instead of only printing them")

	return cmd
}
