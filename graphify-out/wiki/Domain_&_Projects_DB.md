# Domain & Projects DB

> 33 nodes · cohesion 0.12

## Key Concepts

- **projects.go** (10 connections) — `internal/database/projects.go`
- **ListProjects()** (10 connections) — `internal/database/projects.go`
- **newParkCmd()** (9 connections) — `internal/cli/park.go`
- **UpsertProject()** (9 connections) — `internal/database/projects.go`
- **Context** (8 connections) — `internal/database/projects.go`
- **PruneDetectedProjectsUnderPath()** (7 connections) — `internal/database/projects.go`
- **newDomainSetCmd()** (6 connections) — `internal/cli/domain.go`
- **SetPrimaryDomain()** (6 connections) — `internal/database/projects.go`
- **DB** (6 connections) — `internal/database/projects.go`
- **normalizeDomain()** (5 connections) — `internal/cli/naming.go`
- **ensureDomainAvailable()** (5 connections) — `internal/database/projects.go`
- **FindProjectByPath()** (5 connections) — `internal/database/projects.go`
- **TestCustomDomainSurvivesUpsert()** (5 connections) — `internal/database/projects_test.go`
- **TestPruneDetectedProjectsUnderPathRemovesNestedChildren()** (5 connections) — `internal/database/projects_test.go`
- **newDomainCmd()** (4 connections) — `internal/cli/domain.go`
- **primaryDomain()** (4 connections) — `internal/cli/naming.go`
- **currentPrimaryDomain()** (4 connections) — `internal/database/projects.go`
- **InsertPark()** (4 connections) — `internal/database/projects.go`
- **naming.go** (3 connections) — `internal/cli/naming.go`
- **primaryDomainLabel()** (3 connections) — `internal/cli/naming.go`
- **TestNormalizeDomain()** (3 connections) — `internal/cli/naming_test.go`
- **ProjectRecord** (3 connections) — `internal/database/projects.go`
- **domain.go** (2 connections) — `internal/cli/domain.go`
- **placeholders()** (2 connections) — `internal/database/projects.go`
- **projects_test.go** (2 connections) — `internal/database/projects_test.go`
- *... and 8 more nodes in this community*

## Relationships

- [[CLI Commands & Root]] (5 shared connections)
- [[Project Stack Detector]] (1 shared connections)
- [[Compose Runtime & Project Naming]] (1 shared connections)
- [[Observe CLI Commands]] (1 shared connections)
- [[External Proxy Connection]] (1 shared connections)
- [[Proxy Apply (Caddy/Hosts)]] (1 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/cli/domain.go`
- `internal/cli/naming.go`
- `internal/cli/naming_test.go`
- `internal/cli/park.go`
- `internal/database/projects.go`
- `internal/database/projects_test.go`

## Audit Trail

- EXTRACTED: 104 (74%)
- INFERRED: 37 (26%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*