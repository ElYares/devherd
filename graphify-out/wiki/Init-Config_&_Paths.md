# Init/Config & Paths

> 50 nodes · cohesion 0.06

## Key Concepts

- **Config** (15 connections) — `internal/config/config.go`
- **Default()** (12 connections) — `internal/config/config.go`
- **ResolvePaths()** (11 connections) — `internal/config/paths.go`
- **newInitCmd()** (8 connections) — `internal/cli/init.go`
- **newDoctorCmd()** (7 connections) — `internal/cli/doctor.go`
- **applyInitOverrides()** (7 connections) — `internal/cli/init.go`
- **Paths** (7 connections) — `internal/config/paths.go`
- **newServiceActionCmd()** (6 connections) — `internal/cli/service.go`
- **TestApplyInitOverridesKeepsExplicitTLD()** (4 connections) — `internal/cli/init_test.go`
- **TestApplyInitOverridesSetsLocalhostForExternalProxy()** (4 connections) — `internal/cli/init_test.go`
- **service.go** (4 connections) — `internal/cli/service.go`
- **newServiceCmd()** (4 connections) — `internal/cli/service.go`
- **runServiceAction()** (4 connections) — `internal/cli/service.go`
- **Store** (4 connections) — `internal/config/config.go`
- **statusLabel()** (3 connections) — `internal/cli/doctor.go`
- **defaultExternalProxyDir()** (3 connections) — `internal/config/config.go`
- **TestDefaultConfig()** (3 connections) — `internal/config/config_test.go`
- **defaultDataRootForOS()** (3 connections) — `internal/config/paths.go`
- **defaultStateRootForOS()** (3 connections) — `internal/config/paths.go`
- **TestDefaultDataRootForOS()** (3 connections) — `internal/config/paths_test.go`
- **TestDefaultStateRootForOS()** (3 connections) — `internal/config/paths_test.go`
- **Command** (3 connections) — `internal/cli/service.go`
- **doctor.go** (2 connections) — `internal/cli/doctor.go`
- **init.go** (2 connections) — `internal/cli/init.go`
- **init_test.go** (2 connections) — `internal/cli/init_test.go`
- *... and 25 more nodes in this community*

## Relationships

- [[CLI Commands & Root]] (6 shared connections)
- [[Doctor Host Checks]] (4 shared connections)
- [[External Proxy Bootstrap]] (2 shared connections)
- [[External Proxy Connection]] (2 shared connections)
- [[Compose Runtime & Project Naming]] (1 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (1 shared connections)
- [[Preflight & Inspection]] (1 shared connections)

## Source Files

- `internal/cli/doctor.go`
- `internal/cli/init.go`
- `internal/cli/init_test.go`
- `internal/cli/service.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/paths.go`
- `internal/config/paths_test.go`
- `internal/proxy/external.go`

## Audit Trail

- EXTRACTED: 119 (74%)
- INFERRED: 42 (26%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*