# Graph Report - devherd  (2026-06-19)

## Corpus Check
- 126 files · ~67,174 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1275 nodes · 2309 edges · 66 communities (58 shown, 8 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 354 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `296b2694`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Preflight & Inspection|Preflight & Inspection]]
- [[_COMMUNITY_Compose Runtime & Project Naming|Compose Runtime & Project Naming]]
- [[_COMMUNITY_InitConfig & Paths|Init/Config & Paths]]
- [[_COMMUNITY_Doctor Host Checks|Doctor Host Checks]]
- [[_COMMUNITY_CLI Commands & Root|CLI Commands & Root]]
- [[_COMMUNITY_Observe Store & SQLite Models|Observe Store & SQLite Models]]
- [[_COMMUNITY_Observe HTTP Server & Panel|Observe HTTP Server & Panel]]
- [[_COMMUNITY_Observe CLI Commands|Observe CLI Commands]]
- [[_COMMUNITY_External Proxy Connection|External Proxy Connection]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Proxy Apply (CaddyHosts)|Proxy Apply (Caddy/Hosts)]]
- [[_COMMUNITY_Domain & Projects DB|Domain & Projects DB]]
- [[_COMMUNITY_Project Stack Detector|Project Stack Detector]]
- [[_COMMUNITY_Docker CLI Runtime|Docker CLI Runtime]]
- [[_COMMUNITY_Event Normalize & Fingerprint|Event Normalize & Fingerprint]]
- [[_COMMUNITY_Docs Platform & Observe|Docs: Platform & Observe]]
- [[_COMMUNITY_Observe Event Correlation|Observe Event Correlation]]
- [[_COMMUNITY_Compose Observe Override|Compose Observe Override]]
- [[_COMMUNITY_Docs Architecture & Decisions|Docs: Architecture & Decisions]]
- [[_COMMUNITY_Docs Proxy Drivers & Manifest|Docs: Proxy Drivers & Manifest]]
- [[_COMMUNITY_Docs Cross-Platform & Validation|Docs: Cross-Platform & Validation]]
- [[_COMMUNITY_Observe DB Manager|Observe DB Manager]]
- [[_COMMUNITY_Desktop App Manifest|Desktop App Manifest]]
- [[_COMMUNITY_Infra Review Findings|Infra Review Findings]]
- [[_COMMUNITY_Desktop package.json (Vue)|Desktop package.json (Vue)]]
- [[_COMMUNITY_Embedded Templates & Services|Embedded Templates & Services]]
- [[_COMMUNITY_Compose Override Tests|Compose Override Tests]]
- [[_COMMUNITY_install-caddy script|install-caddy script]]
- [[_COMMUNITY_install-ubuntu script|install-ubuntu script]]
- [[_COMMUNITY_uninstall script|uninstall script]]
- [[_COMMUNITY_Stateless No-Daemon Design|Stateless No-Daemon Design]]
- [[_COMMUNITY_observe migrations|observe migrations]]
- [[_COMMUNITY_Go Module Root|Go Module Root]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 63|Community 63]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 79|Community 79]]

## God Nodes (most connected - your core abstractions)
1. `contains()` - 35 edges
2. `Store` - 34 edges
3. `Detect()` - 32 edges
4. `String()` - 32 edges
5. `Project` - 26 edges
6. `newRootCmd()` - 23 edges
7. `loadAppContext()` - 20 edges
8. `ResolveProject()` - 20 edges
9. `Context` - 20 edges
10. `Command` - 19 edges

## Surprising Connections (you probably didn't know these)
- `DevHerd Observe Module` --semantically_similar_to--> `Sentry Templates Placeholder`  [INFERRED] [semantically similar]
  docs/observe.md → templates/sentry/README.md
- `DevHerd` --references--> `Vikunja with DevHerd Guide`  [EXTRACTED]
  README.md → docs/guides/vikunja.md
- `main()` --calls--> `Execute()`  [INFERRED]
  cmd/devherd/main.go → internal/cli/root.go
- `newObserveDetachCmd()` --calls--> `RemoveComposeOverride()`  [INFERRED]
  internal/cli/observe.go → internal/observe/override.go
- `DevHerd` --references--> `Per-Project Usage Workflow`  [EXTRACTED]
  README.md → docs/project-workflow.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **caddy-docker-external Proxy Apply Flow** — architecture_caddy_docker_external, architecture_local_proxy, architecture_devherd_manifest, architecture_sync_hosts [EXTRACTED 1.00]
- **Observe Error Pipeline** — observe_collector, observe_issue_fingerprint, observe_separate_db, observe_docker_correlation, observe_local_alerts [EXTRACTED 1.00]
- **Infra/Quality Improvement Cluster** — improvements_runner_interface, improvements_slog_logging, improvements_docker_runtime, improvements_not_implemented [INFERRED 0.75]

## Communities (66 total, 8 thin omitted)

### Community 0 - "Preflight & Inspection"
Cohesion: 0.08
Nodes (57): Config, Context, Paths, Project, T, T, composeDoc, dockerContainer (+49 more)

### Community 1 - "Compose Runtime & Project Naming"
Cohesion: 0.09
Nodes (46): appendObserveOverride(), prepareComposeProject(), resolveExternalProject(), newDownCmd(), newStopCmd(), manifest, manifestCompose, manifestProxy (+38 more)

### Community 2 - "Init/Config & Paths"
Cohesion: 0.05
Nodes (45): newDoctorCmd(), statusLabel(), applyInitOverrides(), newInitCmd(), TestApplyInitOverridesKeepsExplicitTLD(), TestApplyInitOverridesSetsLocalhostForExternalProxy(), setupLogging(), TestSetupLoggingDoesNotPanic() (+37 more)

### Community 3 - "Doctor Host Checks"
Cohesion: 0.10
Nodes (51): Check, dockerEngineInfo, dockerNetworkInfo, checkBinary(), checkDirectory(), checkDockerCompose(), checkDockerDaemon(), checkDockerEngineMode() (+43 more)

### Community 4 - "CLI Commands & Root"
Cohesion: 0.08
Nodes (61): T, packageJSON, Plan, databaseService(), Detect(), detectPort(), fileExists(), findChild() (+53 more)

### Community 5 - "Observe Store & SQLite Models"
Cohesion: 0.11
Nodes (27): ContainerEvent, ContainerLog, Context, DB, Event, ObservedContainer, Paths, Tx (+19 more)

### Community 6 - "Observe HTTP Server & Panel"
Cohesion: 0.06
Nodes (47): Handler, Server, Request, ResponseWriter, Context, DockerRuntime, Server, Request (+39 more)

### Community 7 - "Observe CLI Commands"
Cohesion: 0.15
Nodes (37): emptyAsAll(), newObserveAlertAddCmd(), newObserveAlertCmd(), newObserveAlertDeliveriesCmd(), newObserveAlertListCmd(), newObserveAlertRemoveCmd(), newObserveAttachCmd(), newObserveCleanupCmd() (+29 more)

### Community 8 - "External Proxy Connection"
Cohesion: 0.06
Nodes (75): browserCommand(), newOpenCmd(), TestBrowserCommand(), externalSettingsConfig, Command, T, Config, Config (+67 more)

### Community 9 - "Community 9"
Cohesion: 0.53
Nodes (5): T, TestCmdRunCapturesOutput(), TestCmdRunRespectsTimeout(), TestCmdRunSetsWorkingDir(), TestCmdRunUsesOutputAsErrorMessage()

### Community 10 - "Proxy Apply (Caddy/Hosts)"
Cohesion: 0.08
Nodes (33): collectDomains(), newProxyApplyCmd(), newProxyBootstrapCmd(), newProxyCmd(), syncManagedDomains(), TestSyncManagedDomainsUsesCollectedDomains(), buildManagedBlock(), mergeManagedBlock() (+25 more)

### Community 11 - "Domain & Projects DB"
Cohesion: 0.12
Nodes (27): newDomainCmd(), newDomainSetCmd(), normalizeDomain(), primaryDomain(), primaryDomainLabel(), TestNormalizeDomain(), newParkCmd(), ProjectRecord (+19 more)

### Community 12 - "Project Stack Detector"
Cohesion: 0.19
Nodes (22): describeFramework(), describeRuntime(), describeStack(), DetectProject(), Discover(), fileExists(), filterNestedProjects(), hasAnyFeature() (+14 more)

### Community 13 - "Docker CLI Runtime"
Cohesion: 0.16
Nodes (17): Context, Duration, Time, T, ContainerEvent, ContainerLog, firstLine(), looksLikeDockerTimestamp() (+9 more)

### Community 14 - "Event Normalize & Fingerprint"
Cohesion: 0.32
Nodes (10): Logs(), LogsArgs(), LogsProject(), TestLogsArgsDefault(), TestLogsArgsWithFollowTailAndServices(), LogsOptions, Context, Project (+2 more)

### Community 15 - "Docs: Platform & Observe"
Cohesion: 0.15
Nodes (17): ProjectNameForPath (SHA1 stable name), Compose Container Isolation Pattern, DevHerd CLI Commands Reference, Current Project Status, Observe Auto-Instrumentation Plan, Observe Docker Label Correlation, Observe Isolation Principle (no prod impact), Issue Fingerprint Grouping (+9 more)

### Community 16 - "Observe Event Correlation"
Cohesion: 0.23
Nodes (11): ContainerLog, Context, DockerRuntime, Event, ObservedContainer, Store, Time, bestContainerMatch() (+3 more)

### Community 17 - "Compose Observe Override"
Cohesion: 0.26
Nodes (12): Project, AttachOptions, AttachResult, composeServicesDoc, observeOverrideDoc, observeOverrideService, BuildComposeOverride(), ComposeServices() (+4 more)

### Community 18 - "Docs: Architecture & Decisions"
Cohesion: 0.17
Nodes (15): loadAppContext (config + SQLite), DevHerd Architecture, Domain Logic Outside cli Package, Committed 21MB Binary Finding, DockerRuntime Dependency Injection, Announced-but-Unimplemented Commands, Architecture & Infra Review, Proposed Runner/Executor Interface (+7 more)

### Community 19 - "Docs: Proxy Drivers & Manifest"
Cohesion: 0.22
Nodes (11): caddy-docker-external Driver, Caddy Host Proxy Driver, Project Stack Detector, .devherd.yml Manifest, local_proxy Managed Container, Proxy Driver Abstraction, DNS SyncHosts (/etc/hosts block), vue+flask Predefined Proxy Routing (+3 more)

### Community 20 - "Docs: Cross-Platform & Validation"
Cohesion: 0.29
Nodes (8): Doctor Host Prerequisites, Preflight Inspect (collision checks), devherd-v1-app Branch, Portable External Proxy Contract, Per-OS Browser Launcher, Per-OS Port Check Strategies, Cross-Platform Runtime Note, Portable Proxy Validation 2026-05-04

### Community 21 - "Observe DB Manager"
Cohesion: 0.20
Nodes (13): appliedVersions(), migrate(), NewManager(), Manager, migration, loadMigrations(), TestEnsureRecordsMigrationsAndIsIdempotent(), TestLoadMigrationsAreOrderedAndNumbered() (+5 more)

### Community 22 - "Desktop App Manifest"
Cohesion: 0.29
Nodes (6): author, authorUrl, description, minAppVersion, name, version

### Community 23 - "Infra Review Findings"
Cohesion: 0.04
Nodes (41): 10. Siguiente bloque recomendado, 1. Que ya funciona, 2. Que hace hoy DevHerd, 3. Que ya validamos en un host real, 4. Que ya validamos manualmente con `local_proxy`, 5. Lo que ya quedo automatizado en `2026-05-04`, 6. Lo que ya validamos en un stack sensible real el `2026-05-04`, 7. Validacion operativa de `aang-server` y `Uniformes` (+33 more)

### Community 24 - "Desktop package.json (Vue)"
Cohesion: 0.40
Nodes (4): dependencies, vue, name, private

### Community 25 - "Embedded Templates & Services"
Cohesion: 0.20
Nodes (10): Embedded Templates (go:embed), Shared Services Manager (Redis/Mailpit), Graphify Knowledge Graph Workflow, Cobra newXxxCmd Command Pattern, DevHerd Contributing Guide, Shared Services Compose Base Template, Observe Local HTTP Collector, Sentry Templates Placeholder (+2 more)

### Community 26 - "Compose Override Tests"
Cohesion: 0.67
Nodes (3): T, TestBuildComposeOverrideObservesSelectedServices(), TestBuildComposeOverrideRejectsMissingService()

### Community 33 - "observe migrations"
Cohesion: 0.17
Nodes (17): Context, Paths, Context, Manager, T, Runner, fakeRunner, Manager (+9 more)

### Community 45 - "Community 45"
Cohesion: 0.04
Nodes (46): 1. Requisitos, 2.1 Durante desarrollo (sin instalar), 2.2 Instalar el binario (Ubuntu), 2.3 Build manual, 2.4 Desinstalar, 2. Instalacion y build, 3. Flags globales, 4.10 `devherd down [path]` (+38 more)

### Community 46 - "Community 46"
Cohesion: 0.04
Nodes (45): 10. Primer MVP alcanzable, 11. Riesgos tecnicos, 12. Buenas practicas de seguridad, 13. Mejoras recomendadas, 14. Base inicial incluida en este repositorio, 1. Vision del producto, 2. Como funcionaria, 3. Requerimientos funcionales (+37 more)

### Community 47 - "Community 47"
Cohesion: 0.12
Nodes (16): Comandos, `devherd doctor`, `devherd domain`, `devherd domain set`, `devherd down`, `devherd init`, `devherd inspect`, `devherd list` (+8 more)

### Community 48 - "Community 48"
Cohesion: 0.06
Nodes (35): 10. Siguiente iteracion recomendada, 1. Alcance, 2. Estado real de la CLI, 3. Prerequisitos, 4. Instalacion del binario, 5. Flujo actual de uso en un proyecto, 6. Flujo recomendado hoy para el proyecto de ejemplo, 7. Flujo actual con `local_proxy` (+27 more)

### Community 49 - "Community 49"
Cohesion: 0.06
Nodes (33): 10. Desactivar Observe en el proyecto, 10. Flujo de uso en un proyecto, 11. Comandos y como funcionan, 12. Criterio de exito del MVP, 1. Arrancar el collector local, 1. Objetivo, 2. Principio de aislamiento, 2. Revisar el DSN local del proyecto (+25 more)

### Community 50 - "Community 50"
Cohesion: 0.08
Nodes (25): 10. DNS local (`/etc/hosts`), 11. Servicios compartidos, 12. Preflight / inspect, 13. Doctor, 14. Observe (observabilidad local), 15. Sentry (estado MVP), 16. Tipos y funciones clave (referencia rapida), 17. Notas de diseno (+17 more)

### Community 51 - "Community 51"
Cohesion: 0.10
Nodes (20): Arquitectura, Calidad de código y testing, Cobertura actual (medida con `go test ./... -cover`), Cómo se usa hoy, DevHerd — Revisión de Arquitectura e Infraestructura, Estado actual, Estado de implementacion (2026-06-19), Formas de usar el proyecto (UX/DX) (+12 more)

### Community 52 - "Community 52"
Cohesion: 0.10
Nodes (19): 1. Entorno de desarrollo, 2. Compilar, 3. Ejecutar tests, 4. Convenciones de codigo observadas, 5. Como agregar un nuevo comando, 6. Como agregar una feature de dominio, 7. Convenciones de Git y entorno, 8. Checklist antes de un PR (+11 more)

### Community 53 - "Community 53"
Cohesion: 0.12
Nodes (14): Comandos nuevos o ajustados, Contrato portable actual, devherd-v1-app, Evidencia, Fases cerradas en esta rama, Objetivo, Pendiente, Alcance (+6 more)

### Community 54 - "Community 54"
Cohesion: 0.13
Nodes (14): 1. Logging estructurado con `slog`, 2. Comando `devherd logs [path]`, 3. Manejo de errores en el colector `observe`, Cómo funciona, Cómo funciona, Cómo ver los logs de diagnóstico, Ejemplos, Ejemplos (+6 more)

### Community 56 - "Community 56"
Cohesion: 0.18
Nodes (10): Alcance de esta pasada, Cambios aplicados, Cross-Platform Runtime 2026-05-05, Docker Desktop y red compartida, `doctor`, Evidencia, Lo que falta, `open` (+2 more)

### Community 57 - "Community 57"
Cohesion: 0.18
Nodes (10): DevHerd Observe: Auto-Instrumentacion, Fase 1: Instrumentar API, Fase 2: Instrumentar Web, Fase 3: Mejorar `observe attach`, Fase 4: Endpoints/Rutas de prueba dev-only, Fase 5: Correlacion con Docker, Fase 6: Panel, Fase 7: Comando de prueba (+2 more)

### Community 59 - "Community 59"
Cohesion: 0.06
Nodes (49): loadAppContext(), appContext, newInspectCmd(), writePreflightReport(), newListCmd(), newLogsCmd(), describeEnvFile(), describeProjectSource() (+41 more)

### Community 69 - "Community 69"
Cohesion: 0.15
Nodes (13): `devherd observe`, `devherd observe attach`, `devherd observe cleanup`, `devherd observe containers`, `devherd observe detach`, `devherd observe dsn`, `devherd observe events`, `devherd observe issues` (+5 more)

### Community 71 - "Community 71"
Cohesion: 0.20
Nodes (9): Comandos futuros probables, Desde donde correr los comandos, DevHerd CLI: Documentacion de Comandos, Ejemplo con el proyecto Vue + Flask + Docker, Estado de la CLI, Existe pero aun no esta implementado, Flujo recomendado de prueba hoy, Funciona hoy (+1 more)

### Community 76 - "Community 76"
Cohesion: 0.40
Nodes (5): `devherd observe alert`, `devherd observe alert add`, `devherd observe alert deliveries`, `devherd observe alert list`, `devherd observe alert remove`

### Community 77 - "Community 77"
Cohesion: 0.50
Nodes (4): `devherd sentry`, `devherd sentry init`, `devherd sentry set-dsn`, `devherd sentry test`

### Community 79 - "Community 79"
Cohesion: 0.67
Nodes (3): `devherd proxy`, `devherd proxy apply`, `devherd proxy bootstrap`

## Knowledge Gaps
- **406 isolated node(s):** `name`, `version`, `minAppVersion`, `description`, `author` (+401 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **8 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `String()` connect `External Proxy Connection` to `Preflight & Inspection`, `Compose Runtime & Project Naming`, `observe migrations`, `Doctor Host Checks`, `CLI Commands & Root`, `Observe Store & SQLite Models`, `Observe HTTP Server & Panel`, `Proxy Apply (Caddy/Hosts)`, `Domain & Projects DB`, `Project Stack Detector`, `Docker CLI Runtime`, `Compose Observe Override`, `Observe DB Manager`, `Community 59`?**
  _High betweenness centrality (0.337) - this node is a cross-community bridge._
- **Why does `BuildComposeOverride()` connect `Compose Observe Override` to `External Proxy Connection`, `Compose Override Tests`, `Observe CLI Commands`?**
  _High betweenness centrality (0.245) - this node is a cross-community bridge._
- **Why does `DevHerd Observe Module` connect `Docs: Platform & Observe` to `Embedded Templates & Services`, `Docs: Architecture & Decisions`, `Docs: Proxy Drivers & Manifest`, `Compose Observe Override`?**
  _High betweenness centrality (0.226) - this node is a cross-community bridge._
- **Are the 32 inferred relationships involving `contains()` (e.g. with `TestVerboseEnablesDebugLevel()` and `normalizeDomain()`) actually correct?**
  _`contains()` has 32 INFERRED edges - model-reasoned connections that need verification._
- **Are the 16 inferred relationships involving `Detect()` (e.g. with `ensureComposeOrScaffold()` and `newScaffoldCmd()`) actually correct?**
  _`Detect()` has 16 INFERRED edges - model-reasoned connections that need verification._
- **Are the 31 inferred relationships involving `String()` (e.g. with `writePreflightReport()` and `newListCmd()`) actually correct?**
  _`String()` has 31 INFERRED edges - model-reasoned connections that need verification._
- **What connects `name`, `version`, `minAppVersion` to the rest of the system?**
  _411 weakly-connected nodes found - possible documentation gaps or missing edges._