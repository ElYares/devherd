# Docker CLI Runtime

> 21 nodes · cohesion 0.16

## Key Concepts

- **docker.go** (11 connections) — `internal/observe/docker.go`
- **.LogsAround()** (7 connections) — `internal/observe/docker.go`
- **runDocker()** (6 connections) — `internal/observe/docker.go`
- **parseDockerLogs()** (5 connections) — `internal/observe/docker.go`
- **.ObservedContainers()** (5 connections) — `internal/observe/docker.go`
- **parseObservedContainers()** (4 connections) — `internal/observe/docker.go`
- **Context** (3 connections) — `internal/observe/docker.go`
- **ContainerLog** (3 connections) — `internal/observe/docker.go`
- **TestParseDockerLogs()** (3 connections) — `internal/observe/docker_test.go`
- **TestParseObservedContainers()** (3 connections) — `internal/observe/docker_test.go`
- **DockerCLI** (3 connections) — `internal/observe/docker.go`
- **ObservedContainer** (3 connections) — `internal/observe/docker.go`
- **T** (2 connections) — `internal/observe/docker_test.go`
- **firstLine()** (2 connections) — `internal/observe/docker.go`
- **looksLikeDockerTimestamp()** (2 connections) — `internal/observe/docker.go`
- **docker_test.go** (2 connections) — `internal/observe/docker_test.go`
- **Duration** (1 connections) — `internal/observe/docker.go`
- **Time** (1 connections) — `internal/observe/docker.go`
- **ContainerEvent** (1 connections) — `internal/observe/docker.go`
- **DockerRuntime** (1 connections) — `internal/observe/docker.go`
- **inspectContainer** (1 connections) — `internal/observe/docker.go`

## Relationships

- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/observe/docker.go`
- `internal/observe/docker_test.go`

## Audit Trail

- EXTRACTED: 64 (93%)
- INFERRED: 5 (7%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*