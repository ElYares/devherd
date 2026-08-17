package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/observe"
	"github.com/spf13/cobra"
)

func newObserveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Collect local development errors and group them into issues",
	}

	cmd.AddCommand(
		newObserveStartCmd(),
		newObserveStatusCmd(),
		newObserveOpenCmd(),
		newObserveDSNCmd(),
		newObserveAttachCmd(),
		newObserveDetachCmd(),
		newObserveScanCmd(),
		newObserveFirewallCmd(),
		newObserveDaemonCmd(),
		newObserveContainersCmd(),
		newObserveTimelineCmd(),
		newObserveCleanupCmd(),
		newObserveAlertCmd(),
		newObserveIssuesCmd(),
		newObserveEventsCmd(),
	)

	return cmd
}

func newObserveStartCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the local Observe collector",
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, store, dbPath, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
				ProxyNetwork: observeSharedNetwork(cmd.Context()),
				Addr:         addr,
				Explicit:     cmd.Flags().Changed("addr"),
			})

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "observe database: %s\n", dbPath)
			for _, bind := range plan.Bind {
				fmt.Fprintf(out, "observe collector: http://%s\n", bind)
			}
			if len(plan.Networks) > 0 && !observe.IsLoopbackAddr(plan.DSN) {
				fmt.Fprintf(out, "containers on %s should use http://%s\n", plan.networkNames(), plan.DSN)
			}
			warnObserveLoopbackDSN(cmd.ErrOrStderr(), plan.DSN, plan)

			server := observe.NewServer(store, dbPath)
			return server.ListenAndServeOn(cmd.Context(), plan.Bind...)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector listen address. Defaults to loopback plus the shared network gateway")

	return cmd
}

func newObserveStatusCmd() *cobra.Command {
	var addr string
	var checkReachability bool

	cmd := &cobra.Command{
		Use:   "status [project]",
		Short: "Check the local Observe collector",
		Long:  "Check the local Observe collector. With a project, the reachability probe runs on that project's own networks instead of a shared one.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var project string
			if len(args) == 1 {
				project = args[0]
			}

			if addr == "" {
				addr = observe.DefaultAddr
			}

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get("http://" + addr + "/health")
			if err != nil {
				return fmt.Errorf("observe collector is not reachable at http://%s: %w", addr, err)
			}
			defer func() { _ = resp.Body.Close() }()

			var payload struct {
				Status   string `json:"status"`
				Database string `json:"database"`
				Error    string `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return fmt.Errorf("decode observe status: %w", err)
			}
			if resp.StatusCode >= 400 {
				if payload.Error == "" {
					payload.Error = resp.Status
				}
				return fmt.Errorf("observe collector is unhealthy: %s", payload.Error)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "observe collector: running at http://%s\n", addr)
			fmt.Fprintf(out, "status: %s\n", payload.Status)
			if payload.Database != "" {
				fmt.Fprintf(out, "database: %s\n", payload.Database)
			}

			if checkReachability {
				plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
					ProxyNetwork: observeSharedNetwork(cmd.Context()),
					Project:      project,
					Addr:         addr,
					Explicit:     cmd.Flags().Changed("addr"),
				})
				reportObserveReachability(cmd, plan)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address")
	cmd.Flags().BoolVar(&checkReachability, "check-reachability", true, "Probe the collector from inside a container on the shared network")

	return cmd
}

func newObserveOpenCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the local Observe panel",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if addr == "" {
				addr = observe.DefaultAddr
			}
			url := "http://" + addr + "/observe"
			name, launcherArgs, ok := browserCommand(runtime.GOOS, url)
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), url)
				return nil
			}
			if _, err := exec.LookPath(name); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), url)
				return nil
			}
			openCmd := exec.Command(name, launcherArgs...)
			if err := openCmd.Start(); err != nil {
				return fmt.Errorf("open observe panel: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address")

	return cmd
}

func newObserveDSNCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "dsn [project]",
		Short: "Print the local DSN for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
				ProxyNetwork: observeSharedNetwork(cmd.Context()),
				Project:      args[0],
				Addr:         addr,
				Explicit:     cmd.Flags().Changed("addr"),
			})
			warnObserveLoopbackDSN(cmd.ErrOrStderr(), plan.DSN, plan)

			fmt.Fprintln(cmd.OutOrStdout(), observeDSN(plan.DSN, args[0]))
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address")

	return cmd
}

func newObserveAttachCmd() *cobra.Command {
	var stack string
	var services []string
	var environment string
	var addr string
	var dsn string
	var dryRun bool
	var reporter bool
	var force bool

	cmd := &cobra.Command{
		Use:   "attach [project-or-path]",
		Short: "Generate a local Observe compose override for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stack = strings.TrimSpace(stack)
			if stack == "" {
				return fmt.Errorf("required flag(s) \"stack\" not set")
			}
			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = app.DB.Close() }()

			target, err := resolveObserveTarget(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}

			plan := planObserveAddrs(cmd.Context(), observeAddrOptions{
				ProxyNetwork: app.Config.Proxy.ExternalNetwork,
				Project:      target.Name,
				Addr:         addr,
				Explicit:     cmd.Flags().Changed("addr"),
			})
			if dsn == "" {
				dsn = observeDSN(plan.DSN, target.Name)
			}
			warnObserveLoopbackDSN(cmd.ErrOrStderr(), dsn, plan)

			options := observe.AttachOptions{
				ProjectName: target.Name,
				Stack:       stack,
				Services:    services,
				DSN:         dsn,
				Environment: environment,
			}

			if dryRun {
				result, err := observe.BuildComposeOverride(target.Compose, options)
				if err != nil {
					return err
				}

				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Observe attach dry run")
				fmt.Fprintf(out, "project: %s\n", target.Name)
				fmt.Fprintf(out, "root: %s\n", target.Compose.Root)
				fmt.Fprintf(out, "stack: %s\n", strings.ToLower(stack))
				fmt.Fprintf(out, "services: %s\n", strings.Join(result.Services, ", "))
				fmt.Fprintf(out, "override: %s\n", result.Path)
				fmt.Fprintln(out, "content:")
				fmt.Fprint(out, result.Content)
				return nil
			}

			result, err := observe.EnsureComposeOverride(target.Compose, options)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "project: %s\n", target.Name)
			fmt.Fprintf(out, "root: %s\n", target.Compose.Root)
			fmt.Fprintf(out, "stack: %s\n", strings.ToLower(stack))
			fmt.Fprintf(out, "services: %s\n", strings.Join(result.Services, ", "))
			fmt.Fprintf(out, "override: %s\n", result.Path)

			if reporter {
				if err := writeObserveReporter(cmd, target.Compose.Root, stack, force); err != nil {
					return err
				}
			}

			fmt.Fprintln(out, "observe attach: complete")
			return nil
		},
	}

	cmd.Flags().StringVar(&stack, "stack", "", "Project stack: laravel, node, python, go, docker or generic")
	cmd.Flags().StringSliceVar(&services, "service", nil, "Compose service to observe; repeat or comma-separate. Defaults to all services")
	cmd.Flags().StringVar(&environment, "environment", "local", "Sentry environment value injected into local override")
	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address used to build the default DSN. Defaults to the shared network gateway so containers can reach the host")
	cmd.Flags().StringVar(&dsn, "dsn", "", "Override the generated local DSN")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the generated override without writing files")
	cmd.Flags().BoolVar(&reporter, "reporter", false, "Also write the reporter that sends events to the collector")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing reporter file")

	return cmd
}

// writeObserveReporter escribe el reporter dentro del proyecto. El override solo
// inyecta el DSN: sin algo que hable con el collector, no sale ni un evento.
func writeObserveReporter(cmd *cobra.Command, root, stack string, force bool) error {
	out := cmd.OutOrStdout()

	result, err := observe.EnsureReporter(root, stack, force)
	switch {
	case errors.Is(err, observe.ErrReporterExists):
		fmt.Fprintf(out, "reporter: %s (kept; pass --force to overwrite)\n", result.Path)
		return nil
	case err != nil:
		return err
	}

	fmt.Fprintf(out, "reporter: %s\n", result.Path)
	if result.Wiring != "" {
		fmt.Fprintf(out, "  wire it up in %s\n", result.Wiring)
	}

	return nil
}

func newObserveDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detach [project-or-path]",
		Short: "Remove the local Observe compose override for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { _ = app.DB.Close() }()

			target, err := resolveObserveTarget(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}

			path, removed, err := observe.RemoveComposeOverride(target.Compose.Root)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "project: %s\n", target.Name)
			fmt.Fprintf(out, "override: %s\n", path)
			if removed {
				fmt.Fprintln(out, "observe detach: removed")
			} else {
				fmt.Fprintln(out, "observe detach: already absent")
			}
			return nil
		},
	}
}

func newObserveScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan [project]",
		Short: "Snapshot observed Docker containers into Observe",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			containers, err := observe.DockerCLI{}.ObservedContainers(cmd.Context(), project)
			if err != nil {
				return err
			}

			events, err := store.StoreContainers(cmd.Context(), containers)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "observed containers: %d\n", len(containers))
			fmt.Fprintf(out, "container events: %d\n", len(events))
			return nil
		},
	}
}

func newObserveContainersCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "containers [project]",
		Short: "List observed Docker containers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			containers, err := store.ListContainers(cmd.Context(), project, limit)
			if err != nil {
				return err
			}
			if len(containers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no observed containers")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "CONTAINER\tPROJECT\tSERVICE\tSTATUS\tRESTARTS\tIMAGE")
			for _, container := range containers {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%d\t%s\n",
					container.Name,
					container.Project,
					container.Service,
					container.Status,
					container.RestartCount,
					container.Image,
				)
			}

			return writer.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum containers to show")

	return cmd
}

func newObserveTimelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "timeline [event-id]",
		Short: "Show the local failure timeline for an event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			timeline, err := store.Timeline(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			writeObserveTimeline(cmd.OutOrStdout(), timeline)
			return nil
		},
	}
}

func newObserveAlertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alert",
		Short: "Manage local Observe alert rules",
	}
	cmd.AddCommand(
		newObserveAlertAddCmd(),
		newObserveAlertListCmd(),
		newObserveAlertRemoveCmd(),
		newObserveAlertDeliveriesCmd(),
	)
	return cmd
}

func newObserveAlertAddCmd() *cobra.Command {
	var project string
	var kind string
	var threshold int
	var window string
	var cooldown string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a local Observe alert rule",
		RunE: func(cmd *cobra.Command, _ []string) error {
			kind = strings.TrimSpace(kind)
			if kind == "" {
				return fmt.Errorf("required flag(s) \"on\" not set")
			}
			if !supportedAlertKind(kind) {
				return fmt.Errorf("unsupported alert kind %q; supported kinds: new-issue, error-rate, container-exit, container-restart", kind)
			}

			windowSeconds, err := parseObserveDurationSeconds(window)
			if err != nil {
				return err
			}

			cooldownSeconds := observe.DefaultCooldownSeconds(kind, windowSeconds)
			if cmd.Flags().Changed("cooldown") {
				cooldownSeconds, err = parseObserveCooldownSeconds(cooldown)
				if err != nil {
					return err
				}
			}

			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			id, err := store.AddAlert(cmd.Context(), observe.Alert{
				Project:         project,
				Kind:            kind,
				Threshold:       threshold,
				WindowSeconds:   windowSeconds,
				CooldownSeconds: cooldownSeconds,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "alert: %d\n", id)
			fmt.Fprintf(out, "project: %s\n", emptyAsAll(project))
			fmt.Fprintf(out, "on: %s\n", kind)
			if kind == "error-rate" {
				fmt.Fprintf(out, "threshold: %d\n", threshold)
				fmt.Fprintf(out, "window: %s\n", window)
			}
			fmt.Fprintf(out, "cooldown: %ds\n", cooldownSeconds)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name; empty applies to all projects")
	cmd.Flags().StringVar(&kind, "on", "", "Alert kind: new-issue, error-rate, container-exit or container-restart")
	cmd.Flags().IntVar(&threshold, "threshold", 1, "Threshold for error-rate alerts")
	cmd.Flags().StringVar(&window, "window", "5m", "Window for error-rate alerts")
	cmd.Flags().StringVar(&cooldown, "cooldown", "", "Silence the rule for this long after it fires; 0 disables the silence (default: the window for error-rate, 15m otherwise)")

	return cmd
}

func newObserveAlertListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [project]",
		Short: "List local Observe alert rules",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			alerts, err := store.ListAlerts(cmd.Context(), project)
			if err != nil {
				return err
			}
			if len(alerts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no observe alerts")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tPROJECT\tON\tTHRESHOLD\tWINDOW\tCOOLDOWN\tENABLED")
			for _, alert := range alerts {
				fmt.Fprintf(writer, "%d\t%s\t%s\t%d\t%ds\t%ds\t%t\n", alert.ID, emptyAsAll(alert.Project), alert.Kind, alert.Threshold, alert.WindowSeconds, alert.CooldownSeconds, alert.Enabled)
			}
			return writer.Flush()
		},
	}
}

func newObserveAlertRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [id]",
		Short: "Remove a local Observe alert rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("alert id must be a positive integer")
			}

			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			removed, err := store.RemoveAlert(cmd.Context(), id)
			if err != nil {
				return err
			}
			if removed {
				fmt.Fprintln(cmd.OutOrStdout(), "observe alert: removed")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "observe alert: not found")
			}
			return nil
		},
	}
}

func newObserveAlertDeliveriesCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "deliveries [project]",
		Short: "List local Observe alert deliveries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			deliveries, err := store.ListAlertDeliveries(cmd.Context(), project, limit)
			if err != nil {
				return err
			}
			if len(deliveries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no observe alert deliveries")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tALERT\tPROJECT\tON\tCREATED\tSUBJECT")
			for _, delivery := range deliveries {
				fmt.Fprintf(writer, "%d\t%d\t%s\t%s\t%s\t%s\n", delivery.ID, delivery.AlertID, delivery.Project, delivery.Kind, delivery.CreatedAt, truncateObserveText(delivery.Subject, 80))
			}
			return writer.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum alert deliveries to show")

	return cmd
}

func newObserveCleanupCmd() *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove old Observe data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			result, err := store.Cleanup(cmd.Context(), days)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "events: %d\n", result.Events)
			fmt.Fprintf(out, "container logs: %d\n", result.ContainerLogs)
			fmt.Fprintf(out, "container events: %d\n", result.ContainerEvents)
			fmt.Fprintf(out, "alert deliveries: %d\n", result.AlertDeliveries)
			fmt.Fprintf(out, "issues: %d\n", result.Issues)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Remove Observe data older than this many days")

	return cmd
}

func newObserveIssuesCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "issues [project]",
		Short: "List grouped Observe issues",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			issues, err := store.ListIssues(cmd.Context(), project, limit)
			if err != nil {
				return err
			}
			if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no observe issues")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tPROJECT\tCOUNT\tSTATUS\tLEVEL\tSERVICE\tLAST SEEN\tTITLE")
			for _, issue := range issues {
				fmt.Fprintf(writer, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
					issue.ID,
					issue.Project,
					issue.EventCount,
					issue.Status,
					issue.Level,
					issue.Service,
					issue.LastSeen,
					truncateObserveText(issue.Title, 90),
				)
			}

			return writer.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum issues to show")

	return cmd
}

func newObserveEventsCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "events [project]",
		Short: "List recent Observe events",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			project := ""
			if len(args) == 1 {
				project = args[0]
			}

			events, err := store.ListEvents(cmd.Context(), project, limit)
			if err != nil {
				return err
			}
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no observe events")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tEVENT\tPROJECT\tISSUE\tLEVEL\tSERVICE\tCONTAINER\tTIMESTAMP\tMESSAGE")
			for _, event := range events {
				fmt.Fprintf(writer, "%d\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
					event.ID,
					event.EventID,
					event.Project,
					event.IssueID,
					event.Level,
					event.Service,
					event.Container,
					event.Timestamp,
					truncateObserveText(event.Message, 90),
				)
			}

			return writer.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum events to show")

	return cmd
}

func writeObserveTimeline(out io.Writer, timeline observe.Timeline) {
	event := timeline.Event
	fmt.Fprintf(out, "Event: %s\n", event.EventID)
	fmt.Fprintf(out, "Project: %s\n", event.Project)
	fmt.Fprintf(out, "Issue: %d\n", event.IssueID)
	fmt.Fprintf(out, "Time: %s\n", event.Timestamp)
	fmt.Fprintf(out, "Level: %s\n", event.Level)
	fmt.Fprintf(out, "Service: %s\n", event.Service)
	fmt.Fprintf(out, "Container: %s\n", event.Container)
	if event.ExceptionType != "" {
		fmt.Fprintf(out, "Exception: %s\n", event.ExceptionType)
	}
	if event.Message != "" {
		fmt.Fprintf(out, "Message: %s\n", event.Message)
	}
	if event.Culprit != "" {
		fmt.Fprintf(out, "Culprit: %s\n", event.Culprit)
	}

	writeObservePayload(out, event.RawPayload)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Container events:")
	if len(timeline.ContainerEvents) == 0 {
		fmt.Fprintln(out, "- none")
	} else {
		for _, item := range timeline.ContainerEvents {
			fmt.Fprintf(out, "- %s %s %s %s\n", item.CreatedAt, item.Kind, item.Status, item.Message)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Container logs:")
	if len(timeline.Logs) == 0 {
		fmt.Fprintln(out, "- none captured")
	} else {
		for _, log := range timeline.Logs {
			fmt.Fprintf(out, "- %s %s\n", log.Timestamp, log.Message)
		}
	}
}

// writeObservePayload muestra las claves del payload crudo que no tienen columna
// propia: `context`, `tags`, breadcrumbs y demas datos que envia un SDK. Se
// guardaban en events.raw_payload desde siempre, pero ninguna consulta las leia.
func writeObservePayload(out io.Writer, rawPayload string) {
	extra := observe.ExtraPayload(rawPayload)
	if len(extra) == 0 {
		return
	}

	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Payload:")
	for _, key := range keys {
		fmt.Fprintf(out, "- %s: %s\n", key, formatObservePayloadValue(extra[key]))
	}
}

func formatObservePayloadValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}

	return string(encoded)
}

type observeTarget struct {
	Name    string
	Compose compose.Project
}

func resolveObserveTarget(ctx context.Context, app *appContext, input string) (observeTarget, error) {
	projects, err := database.ListProjects(ctx, app.DB)
	if err != nil {
		return observeTarget{}, err
	}

	absoluteInput, _ := filepath.Abs(input)
	for _, project := range projects {
		if strings.EqualFold(project.Name, input) || project.Path == absoluteInput {
			composeProject, err := compose.ResolveProject(project.Path)
			if err != nil {
				return observeTarget{}, err
			}

			return observeTarget{Name: project.Name, Compose: composeProject}, nil
		}
	}

	composeProject, err := compose.ResolveProject(input)
	if err != nil {
		return observeTarget{}, fmt.Errorf("project %q was not found as a registered project or compose path: %w", input, err)
	}

	return observeTarget{
		Name:    filepath.Base(composeProject.Root),
		Compose: composeProject,
	}, nil
}

func openObserveStore(cmd *cobra.Command) (*sql.DB, observe.Store, string, error) {
	app, err := loadAppContext(cmd.Context())
	if err != nil {
		return nil, observe.Store{}, "", err
	}
	defer func() { _ = app.DB.Close() }()

	dbPath := observe.DefaultDBPath(app.Paths)
	manager := observe.NewManager(dbPath)
	if _, err := manager.Ensure(cmd.Context()); err != nil {
		return nil, observe.Store{}, "", err
	}

	db, err := manager.Open()
	if err != nil {
		return nil, observe.Store{}, "", err
	}

	return db, observe.NewStore(db), dbPath, nil
}

func observeDSN(addr, project string) string {
	addr = strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://")
	return "http://devherd@" + addr + "/" + url.PathEscape(project)
}

// observeAddrPlan resuelve las direcciones del collector, que no tienen por que
// coincidir: donde escucha en el host y cual se inyecta en los contenedores.
type observeAddrPlan struct {
	Bind     []string
	DSN      string
	Networks []observe.NetworkInfo
	Match    observe.NetworkInfo
	Project  string
	Coverage string
	Reason   string
}

type observeAddrOptions struct {
	ProxyNetwork string
	Project      string
	Addr         string
	Explicit     bool
}

// planObserveAddrs hace escuchar al collector en el gateway de *todas* las redes
// DevHerd, y elige para el DSN el de una red a la que el proyecto este realmente
// conectado. Asumir la red del proxy no vale: a esa solo se conecta el servicio
// que publica el proxy, no el que reporta.
func planObserveAddrs(ctx context.Context, opts observeAddrOptions) observeAddrPlan {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		addr = observe.DefaultAddr
	}

	plan := observeAddrPlan{Bind: []string{addr}, DSN: addr, Project: strings.TrimSpace(opts.Project)}

	// El collector atiende las redes estables de DevHerd y, ademas, las de los
	// contenedores ya observados: la red propia de cada proyecto es justo la que
	// comparten todos sus servicios.
	shared := observe.SharedNetworkNames(opts.ProxyNetwork)
	names := shared
	if observed, err := observe.ObservedNetworks(ctx, nil); err == nil {
		names = append(append([]string{}, shared...), observed...)
	}

	plan.Networks = observe.InspectNetworks(ctx, nil, names)
	if len(plan.Networks) == 0 {
		plan.Reason = "no Docker network could be resolved; is the Docker daemon running?"
		return plan
	}

	plan.Match = plan.selectMatch(ctx, shared)

	// Con --addr explicito manda el usuario: las redes se resuelven solo para
	// poder avisar y para derivar las reglas de cortafuegos.
	if opts.Explicit {
		return plan
	}

	for _, info := range plan.Networks {
		plan.Bind = append(plan.Bind, observe.WithHost(addr, info.Gateway))
	}
	plan.DSN = observe.WithHost(addr, plan.Match.Gateway)

	return plan
}

// selectMatch elige la red del DSN: la que mas contenedores del proyecto
// comparten. Sin proyecto conocido cae a la primera red estable de DevHerd.
func (plan *observeAddrPlan) selectMatch(ctx context.Context, shared []string) observe.NetworkInfo {
	if plan.Project == "" {
		return plan.Networks[0]
	}

	coverage, err := observe.ProjectNetworkCoverage(ctx, nil, plan.Project)
	if err != nil {
		plan.Reason = err.Error()
		return plan.Networks[0]
	}
	if coverage.Containers == 0 {
		plan.Reason = fmt.Sprintf("project %s has no running containers, so its networks are unknown; verify with `devherd observe status %s` after `devherd up`", plan.Project, plan.Project)
		return plan.Networks[0]
	}

	name, count := observe.SelectProjectNetwork(coverage, shared)
	plan.Coverage = fmt.Sprintf("%d/%d", count, coverage.Containers)

	for _, info := range plan.Networks {
		if info.Name == name {
			return info
		}
	}

	// La red del proyecto puede no estar en la lista si sus contenedores no
	// llevan aun la etiqueta de observado: se resuelve a demanda.
	if info, err := observe.InspectNetwork(ctx, nil, name); err == nil {
		plan.Networks = append(plan.Networks, info)
		return info
	}

	return plan.Networks[0]
}

func (plan observeAddrPlan) networkNames() string {
	names := make([]string, 0, len(plan.Networks))
	for _, info := range plan.Networks {
		names = append(names, info.Name)
	}

	return strings.Join(names, ", ")
}

// observeSharedNetwork lee la red compartida de la config y cae al default
// cuando DevHerd no esta inicializado, para que el diagnostico siga funcionando.
func observeSharedNetwork(ctx context.Context) string {
	app, err := loadAppContext(ctx)
	if err != nil {
		return observe.DefaultNetwork
	}
	defer func() { _ = app.DB.Close() }()

	return observeNetworkName(app.Config.Proxy.ExternalNetwork)
}

func observeNetworkName(name string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}

	return observe.DefaultNetwork
}

func warnObserveLoopbackDSN(w io.Writer, dsn string, plan observeAddrPlan) {
	// El aviso tambien aplica cuando el DSN es alcanzable pero el proyecto no
	// esta en ninguna red DevHerd: ahi el problema no es el loopback, es que la
	// direccion elegida no la ve nadie.
	if !observe.IsLoopbackAddr(dsn) {
		if plan.Reason != "" && plan.Project != "" {
			fmt.Fprintf(w, "warning: %s\n", plan.Reason)
			fmt.Fprintf(w, "  connect the project to %s, or run `devherd observe status %s` to verify\n", plan.Match.Name, plan.Project)
		}
		return
	}

	fmt.Fprintf(w, "warning: %s is a loopback address; inside a container it resolves to the container itself\n", dsn)
	if plan.Reason != "" {
		fmt.Fprintf(w, "  %s\n", plan.Reason)
	}
	fmt.Fprintln(w, "  create a DevHerd network with `devherd proxy bootstrap`, or pass the same reachable --addr to `observe start` and `observe attach`")
}

// reportObserveReachability comprueba el collector desde un contenedor, que es
// donde falla de verdad: el host se alcanza siempre a si mismo, aunque el
// cortafuegos descarte el trafico que llega desde la red de Docker. La sonda
// corre en la red del proyecto cuando se conoce, no en una compartida elegida a
// ciegas: probar en la red equivocada da un falso "ok".
func reportObserveReachability(cmd *cobra.Command, plan observeAddrPlan) {
	out := cmd.OutOrStdout()

	if plan.Match.Name == "" {
		reason := plan.Reason
		if reason == "" {
			reason = "no DevHerd network could be resolved"
		}
		fmt.Fprintf(out, "container reachability: skipped (%s)\n", reason)
		return
	}

	result := observe.ProbeFromContainer(cmd.Context(), nil, plan.Match.Name, plan.DSN)
	label := result.Network
	if plan.Project != "" {
		label = plan.Project + " on " + result.Network
	}

	switch {
	case result.Reachable:
		fmt.Fprintf(out, "container reachability (%s): ok at http://%s\n", label, result.Address)
	case result.Skipped:
		fmt.Fprintf(out, "container reachability (%s): skipped (%s)\n", label, result.Reason)
		fmt.Fprintf(out, "  run it manually: %s\n", observe.ProbeCommand(result.Network, plan.DSN, result.Image))
	default:
		fmt.Fprintf(out, "container reachability (%s): FAILED at http://%s\n", label, result.Address)
		if observe.IsLoopbackAddr(plan.DSN) {
			fmt.Fprintln(out, "  the collector address is a loopback one, so containers reach themselves instead of the host")
		}
		if hint := observe.FirewallHint(plan.Match, plan.DSN); hint != "" {
			fmt.Fprintf(out, "  %s\n", hint)
		}
		fmt.Fprintln(out, "  apply the needed rules with `devherd observe firewall --apply`")
	}
}

func truncateObserveText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}

	if limit <= 3 {
		return value[:limit]
	}

	return strings.TrimSpace(value[:limit-3]) + "..."
}

func supportedAlertKind(kind string) bool {
	switch kind {
	case "new-issue", "error-rate", "container-exit", "container-restart":
		return true
	default:
		return false
	}
}

// parseObserveCooldownSeconds no reusa parseObserveDurationSeconds porque aquel
// rechaza el cero, y aqui cero es una opcion legitima: entregar siempre.
func parseObserveCooldownSeconds(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("observe alert cooldown must not be empty")
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse observe alert cooldown: %w", err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("observe alert cooldown must not be negative")
	}

	return int(duration.Seconds()), nil
}

func parseObserveDurationSeconds(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "5m"
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse observe alert window: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("observe alert window must be greater than zero")
	}

	return int(duration.Seconds()), nil
}

func emptyAsAll(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(all)"
	}
	return value
}
