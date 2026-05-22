package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, dbPath, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			if addr == "" {
				addr = observe.DefaultAddr
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "observe database: %s\n", dbPath)
			fmt.Fprintf(out, "observe collector: http://%s\n", addr)

			server := observe.NewServer(store, dbPath)
			return server.ListenAndServe(cmd.Context(), addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector listen address")

	return cmd
}

func newObserveStatusCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the local Observe collector",
		RunE: func(cmd *cobra.Command, args []string) error {
			if addr == "" {
				addr = observe.DefaultAddr
			}

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get("http://" + addr + "/health")
			if err != nil {
				return fmt.Errorf("observe collector is not reachable at http://%s: %w", addr, err)
			}
			defer resp.Body.Close()

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

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address")

	return cmd
}

func newObserveOpenCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the local Observe panel",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if addr == "" {
				addr = observe.DefaultAddr
			}

			fmt.Fprintln(cmd.OutOrStdout(), observeDSN(addr, args[0]))
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

	cmd := &cobra.Command{
		Use:   "attach [project-or-path]",
		Short: "Generate a local Observe compose override for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stack = strings.TrimSpace(stack)
			if stack == "" {
				return fmt.Errorf("required flag(s) \"stack\" not set")
			}
			if addr == "" {
				addr = observe.DefaultAddr
			}

			app, err := loadAppContext(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			target, err := resolveObserveTarget(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}

			if dsn == "" {
				dsn = observeDSN(addr, target.Name)
			}

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
			fmt.Fprintln(out, "observe attach: complete")
			return nil
		},
	}

	cmd.Flags().StringVar(&stack, "stack", "", "Project stack: laravel, node, python, go, docker or generic")
	cmd.Flags().StringSliceVar(&services, "service", nil, "Compose service to observe; repeat or comma-separate. Defaults to all services")
	cmd.Flags().StringVar(&environment, "environment", "local", "Sentry environment value injected into local override")
	cmd.Flags().StringVar(&addr, "addr", observe.DefaultAddr, "Collector address used to build the default DSN")
	cmd.Flags().StringVar(&dsn, "dsn", "", "Override the generated local DSN")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the generated override without writing files")

	return cmd
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
			defer app.DB.Close()

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
			defer db.Close()

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
			defer db.Close()

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
			defer db.Close()

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

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a local Observe alert rule",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			id, err := store.AddAlert(cmd.Context(), observe.Alert{
				Project:       project,
				Kind:          kind,
				Threshold:     threshold,
				WindowSeconds: windowSeconds,
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
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name; empty applies to all projects")
	cmd.Flags().StringVar(&kind, "on", "", "Alert kind: new-issue, error-rate, container-exit or container-restart")
	cmd.Flags().IntVar(&threshold, "threshold", 1, "Threshold for error-rate alerts")
	cmd.Flags().StringVar(&window, "window", "5m", "Window for error-rate alerts")

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
			defer db.Close()

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
			fmt.Fprintln(writer, "ID\tPROJECT\tON\tTHRESHOLD\tWINDOW\tENABLED")
			for _, alert := range alerts {
				fmt.Fprintf(writer, "%d\t%s\t%s\t%d\t%ds\t%t\n", alert.ID, emptyAsAll(alert.Project), alert.Kind, alert.Threshold, alert.WindowSeconds, alert.Enabled)
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
			defer db.Close()

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
			defer db.Close()

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
		RunE: func(cmd *cobra.Command, args []string) error {
			db, store, _, err := openObserveStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

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
			defer db.Close()

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
			defer db.Close()

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
	defer app.DB.Close()

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

func truncateObserveText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}

	if max <= 3 {
		return value[:max]
	}

	return strings.TrimSpace(value[:max-3]) + "..."
}

func supportedAlertKind(kind string) bool {
	switch kind {
	case "new-issue", "error-rate", "container-exit", "container-restart":
		return true
	default:
		return false
	}
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
