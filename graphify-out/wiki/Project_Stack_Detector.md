# Project Stack Detector

> 24 nodes · cohesion 0.19

## Key Concepts

- **detector.go** (17 connections) — `internal/detector/detector.go`
- **DetectProject()** (11 connections) — `internal/detector/detector.go`
- **Discover()** (9 connections) — `internal/detector/detector.go`
- **inspectDirectory()** (6 connections) — `internal/detector/detector.go`
- **featureSet** (6 connections) — `internal/detector/detector.go`
- **filterNestedProjects()** (5 connections) — `internal/detector/detector.go`
- **detector_test.go** (4 connections) — `internal/detector/detector_test.go`
- **Project** (4 connections) — `internal/detector/detector.go`
- **T** (4 connections) — `internal/detector/detector_test.go`
- **describeFramework()** (3 connections) — `internal/detector/detector.go`
- **describeRuntime()** (3 connections) — `internal/detector/detector.go`
- **describeStack()** (3 connections) — `internal/detector/detector.go`
- **fileExists()** (3 connections) — `internal/detector/detector.go`
- **hasAnyFeature()** (3 connections) — `internal/detector/detector.go`
- **packageJSONHasDependency()** (3 connections) — `internal/detector/detector.go`
- **projectRootOwnsChildren()** (3 connections) — `internal/detector/detector.go`
- **shouldSkipDirectory()** (3 connections) — `internal/detector/detector.go`
- **TestDetectExampleProject()** (3 connections) — `internal/detector/detector_test.go`
- **TestDiscoverExamplesDirectory()** (3 connections) — `internal/detector/detector_test.go`
- **TestDiscoverPrefersManagedRootOverNestedChildren()** (3 connections) — `internal/detector/detector_test.go`
- **TestDiscoverSkipsNodeModulesDirectories()** (3 connections) — `internal/detector/detector_test.go`
- **textFileContains()** (3 connections) — `internal/detector/detector.go`
- **isDescendantPath()** (2 connections) — `internal/detector/detector.go`
- **mapsKeys()** (2 connections) — `internal/detector/detector.go`

## Relationships

- [[Compose Runtime & Project Naming]] (1 shared connections)
- [[Domain & Projects DB]] (1 shared connections)
- [[External Proxy Bootstrap]] (1 shared connections)

## Source Files

- `internal/detector/detector.go`
- `internal/detector/detector_test.go`

## Audit Trail

- EXTRACTED: 98 (90%)
- INFERRED: 11 (10%)
- AMBIGUOUS: 0 (0%)

---

*Part of the graphify knowledge wiki. See [[index]] to navigate.*