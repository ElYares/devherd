# Observe DB Manager

> 7 nodes · cohesion 0.33

## Key Concepts

- **Manager** (4 connections) — `internal/database/db.go`
- **db.go** (2 connections) — `internal/database/db.go`
- **NewManager()** (2 connections) — `internal/database/db.go`
- **.Ensure()** (2 connections) — `internal/database/db.go`
- **.Open()** (2 connections) — `internal/database/db.go`
- **Context** (1 connections) — `internal/database/db.go`
- **DB** (1 connections) — `internal/database/db.go`

## Relationships

- No strong cross-community connections detected

## Source Files

- `internal/database/db.go`

## Audit Trail

- EXTRACTED: 14 (100%)
- INFERRED: 0 (0%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*