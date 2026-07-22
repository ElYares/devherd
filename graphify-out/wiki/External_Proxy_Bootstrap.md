# External Proxy Bootstrap

> 38 nodes · cohesion 0.12

## Key Concepts

- **String()** (28 connections) — `internal/version/version.go`
- **Manager** (12 connections) — `internal/services/manager.go`
- **BootstrapExternalProxy()** (9 connections) — `internal/proxy/bootstrap.go`
- **BootstrapExternalProxyWithOptions()** (9 connections) — `internal/proxy/bootstrap.go`
- **T** (7 connections) — `internal/proxy/external_test.go`
- **bootstrap.go** (7 connections) — `internal/proxy/bootstrap.go`
- **bootstrapExternalProxySettings()** (7 connections) — `internal/proxy/bootstrap.go`
- **external_test.go** (7 connections) — `internal/proxy/external_test.go`
- **Context** (6 connections) — `internal/services/manager.go`
- **TestBootstrapExternalProxyWithForceUpdatesManagedFilesButPreservesEnv()** (6 connections) — `internal/proxy/external_test.go`
- **.compose()** (6 connections) — `internal/services/manager.go`
- **.Start()** (6 connections) — `internal/services/manager.go`
- **TestBuildExternalProjectUsesManifestProxy()** (5 connections) — `internal/proxy/external_test.go`
- **writeTestFile()** (5 connections) — `internal/proxy/external_test.go`
- **.Status()** (5 connections) — `internal/services/manager.go`
- **.Stop()** (5 connections) — `internal/services/manager.go`
- **validateService()** (5 connections) — `internal/services/manager.go`
- **renderEmbeddedTemplate()** (4 connections) — `internal/proxy/bootstrap.go`
- **BootstrapResult** (4 connections) — `internal/proxy/bootstrap.go`
- **TestBootstrapExternalProxyCreatesAndReusesFiles()** (4 connections) — `internal/proxy/external_test.go`
- **TestBuildExternalProjectUsesVueFlaskFallback()** (4 connections) — `internal/proxy/external_test.go`
- **.bootstrap()** (4 connections) — `internal/services/manager.go`
- **ensureNetwork()** (4 connections) — `internal/services/manager.go`
- **runDocker()** (4 connections) — `internal/services/manager.go`
- **ensureManagedFile()** (3 connections) — `internal/proxy/bootstrap.go`
- *... and 13 more nodes in this community*

## Relationships

- [[External Proxy Connection]] (9 shared connections)
- [[Doctor Host Checks]] (3 shared connections)
- [[Init/Config & Paths]] (2 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (2 shared connections)
- [[CLI Commands & Root]] (2 shared connections)
- [[Preflight & Inspection]] (2 shared connections)
- [[Compose Runtime & Project Naming]] (1 shared connections)
- [[Domain & Projects DB]] (1 shared connections)
- [[Project Stack Detector]] (1 shared connections)
- [[Docker CLI Runtime]] (1 shared connections)
- [[Event Normalize & Fingerprint]] (1 shared connections)
- [[Compose Observe Override]] (1 shared connections)

## Source Files

- `internal/proxy/bootstrap.go`
- `internal/proxy/external_test.go`
- `internal/services/manager.go`
- `internal/services/manager_test.go`
- `internal/version/version.go`

## Audit Trail

- EXTRACTED: 141 (73%)
- INFERRED: 52 (27%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*