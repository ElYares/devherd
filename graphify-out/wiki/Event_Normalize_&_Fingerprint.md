# Event Normalize & Fingerprint

> 20 nodes · cohesion 0.21

## Key Concepts

- **Event** (16 connections) — `internal/observe/event.go`
- **NormalizeEvent()** (15 connections) — `internal/observe/event.go`
- **mapString()** (5 connections) — `internal/observe/event.go`
- **exceptionDetails()** (4 connections) — `internal/observe/event.go`
- **Fingerprint()** (4 connections) — `internal/observe/event.go`
- **stackCulprit()** (4 connections) — `internal/observe/event.go`
- **T** (3 connections) — `internal/observe/store_test.go`
- **compactJSON()** (3 connections) — `internal/observe/event.go`
- **eventTitle()** (3 connections) — `internal/observe/event.go`
- **firstNonEmpty()** (3 connections) — `internal/observe/event.go`
- **newEventID()** (3 connections) — `internal/observe/event.go`
- **tagValue()** (3 connections) — `internal/observe/event.go`
- **store_test.go** (3 connections) — `internal/observe/store_test.go`
- **TestStoreCreatesAlertDeliveries()** (3 connections) — `internal/observe/store_test.go`
- **TestStoreEventGroupsIssues()** (3 connections) — `internal/observe/store_test.go`
- **logEntryMessage()** (2 connections) — `internal/observe/event.go`
- **normalizeMessage()** (2 connections) — `internal/observe/event.go`
- **stringValue()** (2 connections) — `internal/observe/event.go`
- **timestampValue()** (2 connections) — `internal/observe/event.go`
- **TestStoreContainersRecordsStatusAndRestartEvents()** (2 connections) — `internal/observe/store_test.go`

## Relationships

- [[Observe Store & SQLite Models]] (3 shared connections)
- [[Observe HTTP Server & Panel]] (3 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/observe/event.go`
- `internal/observe/store_test.go`

## Audit Trail

- EXTRACTED: 73 (87%)
- INFERRED: 11 (13%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*