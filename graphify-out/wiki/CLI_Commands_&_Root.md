# CLI Commands & Root

> 43 nodes · cohesion 0.07

## Key Concepts

- **newRootCmd()** (19 connections) — `internal/cli/root.go`
- **loadAppContext()** (18 connections) — `internal/cli/app_context.go`
- **newUpCmd()** (8 connections) — `internal/cli/up.go`
- **newInspectCmd()** (7 connections) — `internal/cli/inspect.go`
- **writePreflightReport()** (6 connections) — `internal/cli/inspect.go`
- **newListCmd()** (6 connections) — `internal/cli/list.go`
- **newPlanCmd()** (6 connections) — `internal/cli/plan.go`
- **newSentryCmd()** (6 connections) — `internal/cli/sentry.go`
- **runUpPreflight()** (6 connections) — `internal/cli/up.go`
- **appContext** (5 connections) — `internal/cli/app_context.go`
- **notImplemented()** (5 connections) — `internal/cli/root.go`
- **newLogsCmd()** (4 connections) — `internal/cli/logs.go`
- **sentry.go** (4 connections) — `internal/cli/sentry.go`
- **newSentryInitCmd()** (4 connections) — `internal/cli/sentry.go`
- **newSentrySetDSNCmd()** (4 connections) — `internal/cli/sentry.go`
- **newSentryTestCmd()** (4 connections) — `internal/cli/sentry.go`
- **Command** (4 connections) — `internal/cli/sentry.go`
- **plan.go** (3 connections) — `internal/cli/plan.go`
- **describeEnvFile()** (3 connections) — `internal/cli/plan.go`
- **describeProjectSource()** (3 connections) — `internal/cli/plan.go`
- **root.go** (3 connections) — `internal/cli/root.go`
- **Execute()** (3 connections) — `internal/cli/root.go`
- **app_context.go** (2 connections) — `internal/cli/app_context.go`
- **inspect.go** (2 connections) — `internal/cli/inspect.go`
- **up.go** (2 connections) — `internal/cli/up.go`
- *... and 18 more nodes in this community*

## Relationships

- [[Compose Runtime & Project Naming]] (8 shared connections)
- [[Init/Config & Paths]] (6 shared connections)
- [[Domain & Projects DB]] (5 shared connections)
- [[Observe CLI Commands]] (4 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (3 shared connections)
- [[External Proxy Connection]] (2 shared connections)
- [[Preflight & Inspection]] (2 shared connections)
- [[External Proxy Bootstrap]] (2 shared connections)

## Source Files

- `cmd/devherd/main.go`
- `internal/cli/app_context.go`
- `internal/cli/inspect.go`
- `internal/cli/list.go`
- `internal/cli/logs.go`
- `internal/cli/plan.go`
- `internal/cli/root.go`
- `internal/cli/sentry.go`
- `internal/cli/up.go`

## Audit Trail

- EXTRACTED: 96 (61%)
- INFERRED: 62 (39%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*