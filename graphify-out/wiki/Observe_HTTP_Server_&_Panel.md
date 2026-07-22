# Observe HTTP Server & Panel

> 40 nodes · cohesion 0.09

## Key Concepts

- **Server** (16 connections) — `internal/observe/server.go`
- **.handleAPI()** (10 connections) — `internal/observe/server.go`
- **writeError()** (7 connections) — `internal/observe/server.go`
- **.handleHealth()** (6 connections) — `internal/observe/server.go`
- **.handlePanelAPI()** (6 connections) — `internal/observe/panel.go`
- **NewServerWithDocker()** (6 connections) — `internal/observe/server.go`
- **writeJSON()** (6 connections) — `internal/observe/server.go`
- **Context** (5 connections) — `internal/observe/server.go`
- **.LogsAround()** (5 connections) — `internal/observe/server_test.go`
- **server_test.go** (5 connections) — `internal/observe/server_test.go`
- **ResponseWriter** (4 connections) — `internal/observe/server.go`
- **T** (4 connections) — `internal/observe/server_test.go`
- **eventPayloadFromEnvelope()** (4 connections) — `internal/observe/server.go`
- **.handlePanel()** (4 connections) — `internal/observe/panel.go`
- **.ListenAndServe()** (4 connections) — `internal/observe/server.go`
- **.pollObservedContainers()** (4 connections) — `internal/observe/server.go`
- **TestEventPayloadFromEnvelope()** (4 connections) — `internal/observe/server_test.go`
- **TestServerServesObservePanelAPI()** (4 connections) — `internal/observe/server_test.go`
- **Request** (3 connections) — `internal/observe/panel.go`
- **Store** (3 connections) — `internal/observe/server.go`
- **fakeDockerRuntime** (3 connections) — `internal/observe/server_test.go`
- **.ObservedContainers()** (3 connections) — `internal/observe/server_test.go`
- **queryInt()** (3 connections) — `internal/observe/panel.go`
- **NewServer()** (3 connections) — `internal/observe/server.go`
- **.snapshotObservedContainers()** (3 connections) — `internal/observe/server.go`
- *... and 15 more nodes in this community*

## Relationships

- [[Event Normalize & Fingerprint]] (3 shared connections)
- [[Observe Event Correlation]] (1 shared connections)
- [[Observe CLI Commands]] (1 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/observe/panel.go`
- `internal/observe/server.go`
- `internal/observe/server_test.go`

## Audit Trail

- EXTRACTED: 131 (87%)
- INFERRED: 20 (13%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*