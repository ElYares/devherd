# Proxy Apply (Caddy/Hosts)

> 37 nodes · cohesion 0.09

## Key Concepts

- **newProxyApplyCmd()** (14 connections) — `internal/cli/proxy.go`
- **Renderer** (8 connections) — `internal/proxy/caddy.go`
- **caddy.go** (7 connections) — `internal/proxy/caddy.go`
- **newProxyBootstrapCmd()** (6 connections) — `internal/cli/proxy.go`
- **syncManagedDomains()** (6 connections) — `internal/cli/proxy.go`
- **SyncHosts()** (6 connections) — `internal/dns/hosts.go`
- **NewRenderer()** (6 connections) — `internal/proxy/caddy.go`
- **proxy.go** (5 connections) — `internal/cli/proxy.go`
- **newProxyCmd()** (5 connections) — `internal/cli/proxy.go`
- **collectDomains()** (4 connections) — `internal/cli/proxy.go`
- **hosts.go** (4 connections) — `internal/dns/hosts.go`
- **mergeManagedBlock()** (4 connections) — `internal/dns/hosts.go`
- **SelectProjects()** (4 connections) — `internal/proxy/caddy.go`
- **TestRenderVueFlaskSite()** (4 connections) — `internal/proxy/caddy_test.go`
- **.projectSite()** (4 connections) — `internal/proxy/caddy.go`
- **TestSyncManagedDomainsUsesCollectedDomains()** (3 connections) — `internal/cli/proxy_test.go`
- **TestMergeManagedBlock()** (3 connections) — `internal/dns/hosts_test.go`
- **Command** (3 connections) — `internal/cli/proxy.go`
- **ProjectRecord** (3 connections) — `internal/proxy/caddy.go`
- **runInteractive()** (3 connections) — `internal/proxy/caddy.go`
- **sudoValidate()** (3 connections) — `internal/proxy/caddy.go`
- **.Apply()** (3 connections) — `internal/proxy/caddy.go`
- **.Render()** (3 connections) — `internal/proxy/caddy.go`
- **Site** (3 connections) — `internal/proxy/caddy.go`
- **buildManagedBlock()** (2 connections) — `internal/dns/hosts.go`
- *... and 12 more nodes in this community*

## Relationships

- [[External Proxy Connection]] (4 shared connections)
- [[CLI Commands & Root]] (3 shared connections)
- [[Compose Runtime & Project Naming]] (2 shared connections)
- [[External Proxy Bootstrap]] (2 shared connections)
- [[Domain & Projects DB]] (1 shared connections)
- [[Init/Config & Paths]] (1 shared connections)

## Source Files

- `internal/cli/proxy.go`
- `internal/cli/proxy_test.go`
- `internal/dns/hosts.go`
- `internal/dns/hosts_test.go`
- `internal/proxy/caddy.go`
- `internal/proxy/caddy_test.go`

## Audit Trail

- EXTRACTED: 106 (80%)
- INFERRED: 27 (20%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*