# Compose Runtime & Project Naming

> 52 nodes · cohesion 0.08

## Key Concepts

- **Project** (26 connections) — `internal/compose/project.go`
- **ResolveProject()** (17 connections) — `internal/compose/project.go`
- **newDownCmd()** (11 connections) — `internal/cli/down.go`
- **prepareComposeProject()** (9 connections) — `internal/cli/compose_runtime.go`
- **resolveExternalProject()** (9 connections) — `internal/cli/compose_runtime.go`
- **UsesDockerExternal()** (9 connections) — `internal/proxy/external.go`
- **newStopCmd()** (8 connections) — `internal/cli/stop.go`
- **composeArgs()** (8 connections) — `internal/compose/project.go`
- **DownProject()** (7 connections) — `internal/compose/project.go`
- **ProjectNameForPath()** (7 connections) — `internal/compose/project.go`
- **StopProject()** (7 connections) — `internal/compose/project.go`
- **project_test.go** (7 connections) — `internal/compose/project_test.go`
- **Context** (7 connections) — `internal/compose/project.go`
- **T** (7 connections) — `internal/compose/project_test.go`
- **Plan()** (6 connections) — `internal/compose/project.go`
- **run()** (6 connections) — `internal/compose/project.go`
- **UpProject()** (6 connections) — `internal/compose/project.go`
- **appendObserveOverride()** (5 connections) — `internal/cli/compose_runtime.go`
- **Down()** (5 connections) — `internal/compose/project.go`
- **TestResolveProjectDefaultsToSingleComposeFile()** (5 connections) — `internal/compose/project_test.go`
- **Up()** (5 connections) — `internal/compose/project.go`
- **LegacyProjectNameForPath()** (4 connections) — `internal/compose/project.go`
- **Stop()** (4 connections) — `internal/compose/project.go`
- **TestPlanReturnsDockerCommand()** (4 connections) — `internal/compose/project_test.go`
- **TestResolveProjectUsesManifestComposeFiles()** (4 connections) — `internal/compose/project_test.go`
- *... and 27 more nodes in this community*

## Relationships

- [[CLI Commands & Root]] (8 shared connections)
- [[External Proxy Connection]] (6 shared connections)
- [[Preflight & Inspection]] (2 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (2 shared connections)
- [[Domain & Projects DB]] (1 shared connections)
- [[Project Stack Detector]] (1 shared connections)
- [[Observe CLI Commands]] (1 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)
- [[Init/Config & Paths]] (1 shared connections)

## Source Files

- `internal/cli/compose_runtime.go`
- `internal/cli/down.go`
- `internal/cli/stop.go`
- `internal/compose/project.go`
- `internal/compose/project_test.go`
- `internal/proxy/external.go`

## Audit Trail

- EXTRACTED: 179 (73%)
- INFERRED: 67 (27%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*