# External Proxy Connection

> 38 nodes · cohesion 0.14

## Key Concepts

- **external.go** (27 connections) — `internal/proxy/external.go`
- **BuildExternalProject()** (12 connections) — `internal/proxy/external.go`
- **ExternalProject** (12 connections) — `internal/proxy/external.go`
- **Config** (11 connections) — `internal/proxy/external.go`
- **ApplyExternalProxy()** (11 connections) — `internal/proxy/external.go`
- **RemoveExternalProxy()** (11 connections) — `internal/proxy/external.go`
- **externalSettings()** (10 connections) — `internal/proxy/external.go`
- **runCommand()** (10 connections) — `internal/proxy/external.go`
- **newOpenCmd()** (9 connections) — `internal/cli/open.go`
- **ConnectProject()** (9 connections) — `internal/proxy/external.go`
- **Context** (7 connections) — `internal/proxy/external.go`
- **ensureExternalProxyReady()** (7 connections) — `internal/proxy/external.go`
- **effectiveDomain()** (6 connections) — `internal/proxy/external.go`
- **ensureExternalProxyNetwork()** (6 connections) — `internal/proxy/external.go`
- **mergeExternalProxyConfig()** (6 connections) — `internal/proxy/external.go`
- **ProjectDomain()** (6 connections) — `internal/proxy/external.go`
- **composeServiceContainer()** (5 connections) — `internal/proxy/external.go`
- **externalProxyContainerName()** (5 connections) — `internal/proxy/external.go`
- **stripManagedDomains()** (5 connections) — `internal/proxy/external.go`
- **browserCommand()** (4 connections) — `internal/cli/open.go`
- **ProjectRecord** (4 connections) — `internal/proxy/external.go`
- **EnsureComposeOverride()** (4 connections) — `internal/proxy/external.go`
- **renderExternalSites()** (4 connections) — `internal/proxy/external.go`
- **externalSettingsConfig** (4 connections) — `internal/proxy/external.go`
- **TestBrowserCommand()** (3 connections) — `internal/cli/open_test.go`
- *... and 13 more nodes in this community*

## Relationships

- [[External Proxy Bootstrap]] (9 shared connections)
- [[Compose Runtime & Project Naming]] (6 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (4 shared connections)
- [[CLI Commands & Root]] (2 shared connections)
- [[Init/Config & Paths]] (2 shared connections)
- [[Observe CLI Commands]] (1 shared connections)
- [[Domain & Projects DB]] (1 shared connections)
- [[Preflight & Inspection]] (1 shared connections)

## Source Files

- `internal/cli/open.go`
- `internal/cli/open_test.go`
- `internal/proxy/external.go`

## Audit Trail

- EXTRACTED: 197 (87%)
- INFERRED: 29 (13%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*