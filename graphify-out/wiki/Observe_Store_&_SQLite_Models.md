# Observe Store & SQLite Models

> 42 nodes · cohesion 0.11

## Key Concepts

- **Store** (34 connections) — `internal/observe/store.go`
- **Context** (20 connections) — `internal/observe/store.go`
- **.StoreEvent()** (9 connections) — `internal/observe/store.go`
- **insertContainerAlertDeliveries()** (7 connections) — `internal/observe/store.go`
- **insertEventAlertDeliveries()** (7 connections) — `internal/observe/store.go`
- **.StoreContainers()** (7 connections) — `internal/observe/store.go`
- **Tx** (6 connections) — `internal/observe/store.go`
- **insertAlertDelivery()** (6 connections) — `internal/observe/store.go`
- **matchingAlerts()** (6 connections) — `internal/observe/store.go`
- **Alert** (5 connections) — `internal/observe/store.go`
- **Timeline** (5 connections) — `internal/observe/store.go`
- **ContainerEvent** (4 connections) — `internal/observe/store.go`
- **Manager** (4 connections) — `internal/observe/store.go`
- **.Cleanup()** (4 connections) — `internal/observe/store.go`
- **containerEventsForSnapshot()** (4 connections) — `internal/observe/store.go`
- **execDelete()** (4 connections) — `internal/observe/store.go`
- **issueIsNew()** (4 connections) — `internal/observe/store.go`
- **DB** (3 connections) — `internal/observe/store.go`
- **ObservedContainer** (3 connections) — `internal/observe/store.go`
- **EventRecord** (3 connections) — `internal/observe/store.go`
- **.Ensure()** (3 connections) — `internal/observe/store.go`
- **.Open()** (3 connections) — `internal/observe/store.go`
- **.AddAlert()** (3 connections) — `internal/observe/store.go`
- **DefaultDBPath()** (3 connections) — `internal/observe/store.go`
- **.ListAlertDeliveries()** (3 connections) — `internal/observe/store.go`
- *... and 17 more nodes in this community*

## Relationships

- [[Event Normalize & Fingerprint]] (3 shared connections)
- [[Observe CLI Commands]] (1 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/observe/store.go`

## Audit Trail

- EXTRACTED: 193 (97%)
- INFERRED: 5 (3%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*