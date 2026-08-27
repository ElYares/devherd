# Graph Report - devherd  (2026-08-26)

## Corpus Check
- 162 files · ~109,520 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1757 nodes · 3458 edges · 90 communities (80 shown, 10 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 553 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `9f514f7f`
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
- [[_COMMUNITY_db migrations|db migrations]]
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
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 62|Community 62]]
- [[_COMMUNITY_Community 63|Community 63]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 68|Community 68]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 70|Community 70]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 73|Community 73]]
- [[_COMMUNITY_Community 74|Community 74]]
- [[_COMMUNITY_Community 75|Community 75]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 78|Community 78]]
- [[_COMMUNITY_Community 79|Community 79]]
- [[_COMMUNITY_Community 80|Community 80]]
- [[_COMMUNITY_Community 81|Community 81]]
- [[_COMMUNITY_Community 82|Community 82]]
- [[_COMMUNITY_Community 83|Community 83]]
- [[_COMMUNITY_Community 84|Community 84]]
- [[_COMMUNITY_Community 85|Community 85]]
- [[_COMMUNITY_Community 86|Community 86]]
- [[_COMMUNITY_Community 87|Community 87]]
- [[_COMMUNITY_Community 88|Community 88]]

## God Nodes (most connected - your core abstractions)
1. `String()` - 44 edges
2. `Store` - 43 edges
3. `contains()` - 35 edges
4. `Detect()` - 33 edges
5. `Project` - 29 edges
6. `runCoverageCmd()` - 25 edges
7. `newRootCmd()` - 24 edges
8. `planObserveAddrs()` - 23 edges
9. `Context` - 23 edges
10. `loadAppContext()` - 22 edges

## Surprising Connections (you probably didn't know these)
- `DevHerd Observe Module` --semantically_similar_to--> `Sentry Templates Placeholder`  [INFERRED] [semantically similar]
  docs/observe.md → templates/sentry/README.md
- `DevHerd` --references--> `Vikunja with DevHerd Guide`  [EXTRACTED]
  README.md → docs/guides/vikunja.md
- `main()` --calls--> `Execute()`  [INFERRED]
  cmd/devherd/main.go → internal/cli/root.go
- `newObserveDetachCmd()` --calls--> `RemoveComposeOverride()`  [INFERRED]
  internal/cli/observe.go → internal/observe/override.go
- `DevHerd` --references--> `DevHerd Observe Module`  [EXTRACTED]
  README.md → docs/observe.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **caddy-docker-external Proxy Apply Flow** — architecture_caddy_docker_external, architecture_local_proxy, architecture_devherd_manifest, architecture_sync_hosts [EXTRACTED 1.00]
- **Observe Error Pipeline** — observe_collector, observe_issue_fingerprint, observe_separate_db, observe_docker_correlation, observe_local_alerts [EXTRACTED 1.00]
- **Infra/Quality Improvement Cluster** — improvements_runner_interface, improvements_slog_logging, improvements_docker_runtime, improvements_not_implemented [INFERRED 0.75]

## Communities (90 total, 10 thin omitted)

### Community 0 - "Preflight & Inspection"
Cohesion: 0.08
Nodes (57): Config, Context, Paths, Project, T, T, composeDoc, dockerContainer (+49 more)

### Community 1 - "Compose Runtime & Project Naming"
Cohesion: 0.16
Nodes (26): newDownCmd(), manifestCompose, manifestProxy, manifestTest, Project, Command(), composeArgs(), Down() (+18 more)

### Community 2 - "Init/Config & Paths"
Cohesion: 0.10
Nodes (25): newServiceActionCmd(), newServiceCmd(), runServiceAction(), serviceActionShort(), Config, defaultExternalProxyDir(), NewStore(), DNSConfig (+17 more)

### Community 3 - "Doctor Host Checks"
Cohesion: 0.10
Nodes (51): Check, dockerEngineInfo, dockerNetworkInfo, checkBinary(), checkDirectory(), checkDockerCompose(), checkDockerDaemon(), checkDockerEngineMode() (+43 more)

### Community 4 - "CLI Commands & Root"
Cohesion: 0.07
Nodes (69): confirm(), ensureComposeOrScaffold(), newScaffoldCmd(), promptDatabase(), Command, T, packageJSON, Plan (+61 more)

### Community 5 - "Observe Store & SQLite Models"
Cohesion: 0.10
Nodes (33): ContainerEvent, ContainerLog, Context, DB, Event, ObservedContainer, Paths, Time (+25 more)

### Community 6 - "Observe HTTP Server & Panel"
Cohesion: 0.07
Nodes (39): Handler, Server, T, Server, Request, ResponseWriter, Context, DockerRuntime (+31 more)

### Community 7 - "Observe CLI Commands"
Cohesion: 0.05
Nodes (88): loadAppContext(), appContext, TestDescribeEnvFile(), TestDescribeProjectSource(), TestEmptyAsAll(), TestFormatObservePayloadValue(), TestObserveCooldownAcceptsZeroButWindowDoesNot(), TestParseObserveDurationSeconds() (+80 more)

### Community 8 - "External Proxy Connection"
Cohesion: 0.08
Nodes (58): browserCommand(), newOpenCmd(), TestBrowserCommand(), collectDomains(), newProxyApplyCmd(), newProxyBootstrapCmd(), newProxyCmd(), resolveExternalProjects() (+50 more)

### Community 9 - "Community 9"
Cohesion: 0.16
Nodes (34): projectWithReports(), TestCoverageDiscoversTheReportWithoutFlags(), TestCoverageDiscoveryFeedsTheStructuralAnalysis(), TestCoverageDiscoveryMarksTheManagedReport(), TestCoverageDiscoveryMentionsTheReportItDidNotUse(), TestCoverageExplicitReportSkipsDiscovery(), goModuleFixture(), TestCoverageStructureFailsWithoutAModule() (+26 more)

### Community 10 - "Proxy Apply (Caddy/Hosts)"
Cohesion: 0.18
Nodes (12): Config, Paths, ProjectRecord, T, NewRenderer(), runInteractive(), SelectProjects(), sudoValidate() (+4 more)

### Community 11 - "Domain & Projects DB"
Cohesion: 0.12
Nodes (27): newDomainCmd(), newDomainSetCmd(), normalizeDomain(), primaryDomain(), primaryDomainLabel(), TestNormalizeDomain(), newParkCmd(), ProjectRecord (+19 more)

### Community 12 - "Project Stack Detector"
Cohesion: 0.19
Nodes (22): describeFramework(), describeRuntime(), describeStack(), DetectProject(), Discover(), fileExists(), filterNestedProjects(), hasAnyFeature() (+14 more)

### Community 13 - "Docker CLI Runtime"
Cohesion: 0.18
Nodes (17): Context, Duration, Time, T, ContainerEvent, ContainerLog, firstLine(), looksLikeDockerTimestamp() (+9 more)

### Community 14 - "Event Normalize & Fingerprint"
Cohesion: 0.23
Nodes (11): buildManagedBlock(), mergeManagedBlock(), runInteractive(), SyncHosts(), TestMergeManagedBlock(), validateDomains(), TestValidateDomainsAcceptsValid(), TestValidateDomainsNormalizesAndDedupes() (+3 more)

### Community 15 - "Docs: Platform & Observe"
Cohesion: 0.18
Nodes (12): ProjectNameForPath (SHA1 stable name), Compose Container Isolation Pattern, DevHerd CLI Commands Reference, Current Project Status, DevHerd CLI (Go + Cobra), DevHerd, Documentacion, Estado actual (+4 more)

### Community 16 - "Observe Event Correlation"
Cohesion: 0.05
Nodes (39): 10. Riesgos y deuda priorizados, 11. Fortalezas que conviene preservar, 12. Siguiente bloque recomendado, 1. Qué es DevHerd, 2. Mapa de subsistemas, 3.1 Contexto de aplicación, 3.2 Rutas XDG, 3.3 Persistencia (+31 more)

### Community 17 - "Compose Observe Override"
Cohesion: 0.26
Nodes (12): Project, AttachOptions, AttachResult, composeServicesDoc, observeOverrideDoc, observeOverrideService, BuildComposeOverride(), ComposeServices() (+4 more)

### Community 18 - "Docs: Architecture & Decisions"
Cohesion: 0.14
Nodes (18): loadAppContext (config + SQLite), DevHerd Architecture, Graphify Knowledge Graph Workflow, Cobra newXxxCmd Command Pattern, Domain Logic Outside cli Package, DevHerd Contributing Guide, Committed 21MB Binary Finding, DockerRuntime Dependency Injection (+10 more)

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
Cohesion: 0.09
Nodes (23): 1. Comandos disponibles, 2. Que hace hoy DevHerd, 3. Estado de ingenieria (medido), 4. Limitaciones actuales, 5. Siguiente bloque recomendado, Ciclo de vida, Cobertura y verificacion, Codigo muerto (+15 more)

### Community 24 - "Desktop package.json (Vue)"
Cohesion: 0.40
Nodes (4): dependencies, vue, name, private

### Community 25 - "Embedded Templates & Services"
Cohesion: 0.18
Nodes (13): Embedded Templates (go:embed), Shared Services Manager (Redis/Mailpit), Shared Services Compose Base Template, Observe Auto-Instrumentation Plan, Observe Local HTTP Collector, Observe Docker Label Correlation, Observe Isolation Principle (no prod impact), Issue Fingerprint Grouping (+5 more)

### Community 26 - "Compose Override Tests"
Cohesion: 0.67
Nodes (3): T, TestBuildComposeOverrideObservesSelectedServices(), TestBuildComposeOverrideRejectsMissingService()

### Community 33 - "observe migrations"
Cohesion: 0.06
Nodes (59): externalSettingsConfig, T, Config, T, T, Context, Paths, Runner (+51 more)

### Community 37 - "db migrations"
Cohesion: 0.12
Nodes (31): ContainerLog, Context, DockerRuntime, Duration, ObservedContainer, Server, Store, T (+23 more)

### Community 45 - "Community 45"
Cohesion: 0.15
Nodes (13): 4.17 `devherd observe ...`, `observe alert <add|list|remove|deliveries>`, `observe attach <project-or-path> --stack <stack>`, `observe cleanup`, `observe daemon install|uninstall|status`, `observe detach <project-or-path>`, `observe dsn <project>`, `observe firewall` (+5 more)

### Community 46 - "Community 46"
Cohesion: 0.04
Nodes (45): 10. Primer MVP alcanzable, 11. Riesgos tecnicos, 12. Buenas practicas de seguridad, 13. Mejoras recomendadas, 14. Base inicial incluida en este repositorio, 1. Vision del producto, 2. Como funcionaria, 3. Requerimientos funcionales (+37 more)

### Community 47 - "Community 47"
Cohesion: 0.10
Nodes (30): Context, NetworkInfo, T, Context, NetworkInfo, Runner, T, ApplyFirewallRules() (+22 more)

### Community 48 - "Community 48"
Cohesion: 0.06
Nodes (33): 10. Siguiente iteracion recomendada, 1. Alcance, 2. Estado de la CLI, 3. Prerequisitos, 4. Instalacion del binario, 5. Flujo actual de uso en un proyecto, 6. Flujo recomendado hoy para el proyecto de ejemplo, 7. Flujo actual con `local_proxy` (+25 more)

### Community 49 - "Community 49"
Cohesion: 0.05
Nodes (40): 10. Desactivar Observe en el proyecto, 10. Flujo de uso en un proyecto, 11. Comandos y como funcionan, 12. Criterio de exito del MVP, 1. Arrancar el collector local, 1. Objetivo, 2. Principio de aislamiento, 2. Revisar el DSN local del proyecto (+32 more)

### Community 50 - "Community 50"
Cohesion: 0.07
Nodes (28): 10. DNS local (`/etc/hosts`), 11. Servicios compartidos, 12. Preflight / inspect, 13. Doctor, 14. Observe (observabilidad local), 15. Sentry (placeholder), 16. Tipos y funciones clave (referencia rapida), 17. Notas de diseno (+20 more)

### Community 51 - "Community 51"
Cohesion: 0.07
Nodes (27): Alertas: sin silenciamiento, ruido garantizado, Arquitectura, Bloqueantes: la ingesta desde contenedores no funciona de fabrica, Calidad de código y testing, Captura de logs: de una foto a dos pasadas, Cobertura (medida con `go test ./... -cover`), Cómo se usa hoy, DevHerd — Revisión de Arquitectura e Infraestructura (+19 more)

### Community 52 - "Community 52"
Cohesion: 0.11
Nodes (19): 1. Entorno de desarrollo, 2. Compilar, 3. Ejecutar tests, 4. Convenciones de codigo observadas, 5. Como agregar un nuevo comando, 6. Como agregar una feature de dominio, 7. Convenciones de Git y entorno, 8. Checklist antes de un PR (+11 more)

### Community 53 - "Community 53"
Cohesion: 0.12
Nodes (14): Comandos nuevos o ajustados, Contrato portable actual, devherd-v1-app, Evidencia, Fases cerradas en esta rama, Objetivo, Pendiente, Alcance (+6 more)

### Community 54 - "Community 54"
Cohesion: 0.14
Nodes (14): 1. Logging estructurado con `slog`, 2. Comando `devherd logs [path]`, 3. Manejo de errores en el colector `observe`, Cómo funciona, Cómo funciona, Cómo ver los logs de diagnóstico, Ejemplos, Ejemplos (+6 more)

### Community 55 - "Community 55"
Cohesion: 0.11
Nodes (39): countCoverageUncoveredFiles(), coverageStackOrEmpty(), firstArg(), newCoverageCmd(), coverageProjectRoot(), coverageStack(), firstNonEmptyCoverage(), lastLines() (+31 more)

### Community 56 - "Community 56"
Cohesion: 0.18
Nodes (10): Alcance de esta pasada, Cambios aplicados, Cross-Platform Runtime 2026-05-05, Docker Desktop y red compartida, `doctor`, Evidencia, Lo que falta, `open` (+2 more)

### Community 57 - "Community 57"
Cohesion: 0.18
Nodes (11): DevHerd Observe: Auto-Instrumentacion, Estado por fase (revisado 2026-07-21), Fase 1: Instrumentar API, Fase 2: Instrumentar Web, Fase 3: Mejorar `observe attach`, Fase 4: Endpoints/Rutas de prueba dev-only, Fase 5: Correlacion con Docker, Fase 6: Panel (+3 more)

### Community 58 - "Community 58"
Cohesion: 0.20
Nodes (10): Archivos generados, Bases de datos y Redis, Detección fina, Flags, Laravel (soporte completo), Oferta automática desde `up`, Scaffolding de Docker para repos sin contenedores, Sin colisiones con tus proyectos levantados (+2 more)

### Community 59 - "Community 59"
Cohesion: 0.29
Nodes (11): composeProjectLabel(), LegacyProjectNameForPath(), ProjectNameForPath(), TestComposeArgsIncludesEnvFileAndAllComposeFiles(), TestPlanReturnsDockerCommand(), TestProjectNameForPathIsStableAndPathScoped(), TestResolveProjectDefaultsToSingleComposeFile(), TestResolveProjectErrorsWhenManifestComposeFileMissing() (+3 more)

### Community 60 - "Community 60"
Cohesion: 0.22
Nodes (8): 1. Crear el proyecto, 2. Levantarlo con DevHerd, 3. Abrir Vikunja, 4. Tumbarlo cuando no lo necesites, 5. Reanudarlo, 6. Comandos rapidos, 7. Nota sobre datos, Vikunja con DevHerd

### Community 62 - "Community 62"
Cohesion: 0.12
Nodes (17): 4.10 `devherd up [path]`, 4.11 `devherd serve [path]`, 4.12 `devherd stop [path]`, 4.13 `devherd down [path]`, 4.14 `devherd open <project>`, 4.15 `devherd logs [path]`, 4.16 `devherd service <start|stop|status> [service]`, 4.18 `devherd coverage` (+9 more)

### Community 66 - "Community 66"
Cohesion: 0.11
Nodes (35): Context, Runner, Context, T, T, fakeRunner, InspectNetwork(), InspectNetworks() (+27 more)

### Community 67 - "Community 67"
Cohesion: 0.23
Nodes (12): ContainerLog, Context, DockerRuntime, Event, ObservedContainer, Store, Time, bestContainerMatch() (+4 more)

### Community 68 - "Community 68"
Cohesion: 0.10
Nodes (20): 1. La unidad va siempre en la cabecera, 2. El total se pondera por unidades, nunca se promedian porcentajes, 3. Se ordena por masa sin cubrir, no por porcentaje, Como generar el reporte, Detalles que conviene saber, DevHerd Coverage, Encontrar el reporte solo, Estado (+12 more)

### Community 69 - "Community 69"
Cohesion: 0.14
Nodes (14): 10. Trampas conocidas, 1. Requisitos de red (leelo antes que nada), 2. Aplicar el attach, 3. El reporter, 4. Cablearlo (Laravel 11+), 5. Que se captura solo, 6. Que NO se captura, 7. Reportar a proposito (+6 more)

### Community 71 - "Community 71"
Cohesion: 0.32
Nodes (10): Logs(), LogsArgs(), LogsProject(), TestLogsArgsDefault(), TestLogsArgsWithFollowTailAndServices(), LogsOptions, Context, Project (+2 more)

### Community 72 - "Community 72"
Cohesion: 0.22
Nodes (9): 1. Requisitos, 3. Flags globales, 5. El manifiesto `.devherd.yml`, 7. Donde vive el estado, 8.1 Aislamiento de nombres de contenedor, 8.2 Volumenes que sobreviven a cambios de project-name, 8.3 Probar sin tocar tu configuracion real, 8. Patrones recomendados para proyectos Compose (+1 more)

### Community 73 - "Community 73"
Cohesion: 0.29
Nodes (7): Automatizado en `2026-05-04`, Flujo manual con `local_proxy` (previo a la automatizacion), Historial de validaciones, Stack sensible real (`2026-05-04`), Validacion inicial (proyecto de ejemplo), Validacion operativa de `aang-server` y `Uniformes`, Validacion operativa pendiente

### Community 74 - "Community 74"
Cohesion: 0.24
Nodes (9): newInspectCmd(), writePreflightReport(), newUpCmd(), runUpPreflight(), Command, Report, Writer, appContext (+1 more)

### Community 75 - "Community 75"
Cohesion: 0.33
Nodes (6): 6.1 Modo proxy en Docker externo (recomendado), 6.2 Modo proxy en host (Caddy + /etc/hosts), 6.3 Repositorio sin Docker, 6.4 Servicios compartidos + observabilidad, 6.5 Bajar todo, 6. Flujos de trabajo

### Community 76 - "Community 76"
Cohesion: 0.22
Nodes (9): 1. Arquitectura de componentes, 2. Flujo típico de usuario, 3. `devherd up` paso a paso, 4. Proxy: driver `caddy-docker-external`, 5. Observe: ingesta de eventos, 6. Modelo de datos, 7. Estados de un proyecto, Cómo regenerar / editar (+1 more)

### Community 77 - "Community 77"
Cohesion: 0.40
Nodes (5): 2.1 Durante desarrollo (sin instalar), 2.2 Instalar el binario (Ubuntu), 2.3 Build manual, 2.4 Desinstalar, 2. Instalacion y build

### Community 78 - "Community 78"
Cohesion: 0.20
Nodes (18): newObserveDaemonCmd(), newObserveDaemonInstallCmd(), newObserveDaemonStatusCmd(), newObserveDaemonUninstallCmd(), Command, Context, T, DaemonStatus() (+10 more)

### Community 79 - "Community 79"
Cohesion: 0.67
Nodes (3): DevHerd CLI: referencia de comandos, Donde esta ahora cada cosa, Por que se consolido

### Community 80 - "Community 80"
Cohesion: 0.67
Nodes (3): 4.19 `devherd sentry ...`, `sentry init <project> --stack <stack>`, `sentry set-dsn <project> --dsn <dsn>` / `sentry test <project>`

### Community 81 - "Community 81"
Cohesion: 0.67
Nodes (3): 4.6 `devherd proxy`, `devherd proxy apply [project]`, `devherd proxy bootstrap`

### Community 82 - "Community 82"
Cohesion: 0.53
Nodes (5): describeEnvFile(), describeProjectSource(), newPlanCmd(), Command, Project

### Community 83 - "Community 83"
Cohesion: 0.07
Nodes (58): appendObserveOverride(), prepareComposeProject(), resolveExternalProject(), externalProxyConfig(), newComposeProjectDir(), newTestAppContext(), seedProject(), TestAppendObserveOverrideOnlyWhenTheFileExists() (+50 more)

### Community 84 - "Community 84"
Cohesion: 0.38
Nodes (8): newLogsCmd(), notImplemented(), newSentryCmd(), newSentryInitCmd(), newSentrySetDSNCmd(), newSentryTestCmd(), Command, Command

### Community 85 - "Community 85"
Cohesion: 0.33
Nodes (10): newServeCmd(), projectNameForPath(), runSiblingCommand(), newTestRoot(), TestRunSiblingCommandInvokesTargetWithArgs(), TestRunSiblingCommandPropagatesError(), TestRunSiblingCommandResolvesSubcommand(), Command (+2 more)

### Community 86 - "Community 86"
Cohesion: 0.13
Nodes (12): newDoctorCmd(), statusLabel(), newListCmd(), Execute(), newRootCmd(), newStopCmd(), main(), Command (+4 more)

### Community 87 - "Community 87"
Cohesion: 0.07
Nodes (56): Duration, Store, T, Time, T, Event, T, T (+48 more)

### Community 88 - "Community 88"
Cohesion: 0.50
Nodes (4): manifest, manifestCompose, manifestProxy, manifestTest

## Knowledge Gaps
- **489 isolated node(s):** `name`, `version`, `minAppVersion`, `description`, `author` (+484 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **10 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `String()` connect `observe migrations` to `Preflight & Inspection`, `Compose Runtime & Project Naming`, `Doctor Host Checks`, `CLI Commands & Root`, `Observe Store & SQLite Models`, `Observe HTTP Server & Panel`, `Observe CLI Commands`, `External Proxy Connection`, `Community 9`, `Domain & Projects DB`, `Project Stack Detector`, `Docker CLI Runtime`, `Event Normalize & Fingerprint`, `Compose Observe Override`, `Observe DB Manager`, `Community 47`, `Community 55`, `Community 74`, `Community 78`, `Community 86`, `Community 87`?**
  _High betweenness centrality (0.407) - this node is a cross-community bridge._
- **Why does `DevHerd Observe Module` connect `Embedded Templates & Services` to `Compose Observe Override`, `Docs: Architecture & Decisions`, `Docs: Proxy Drivers & Manifest`, `Docs: Platform & Observe`?**
  _High betweenness centrality (0.385) - this node is a cross-community bridge._
- **Why does `DevHerd` connect `Docs: Platform & Observe` to `Embedded Templates & Services`, `Docs: Architecture & Decisions`, `Docs: Proxy Drivers & Manifest`, `Community 61`?**
  _High betweenness centrality (0.385) - this node is a cross-community bridge._
- **Are the 43 inferred relationships involving `String()` (e.g. with `warnCoverageReportNotIgnored()` and `writeCoverageFunctions()`) actually correct?**
  _`String()` has 43 INFERRED edges - model-reasoned connections that need verification._
- **Are the 32 inferred relationships involving `contains()` (e.g. with `TestVerboseEnablesDebugLevel()` and `normalizeDomain()`) actually correct?**
  _`contains()` has 32 INFERRED edges - model-reasoned connections that need verification._
- **Are the 17 inferred relationships involving `Detect()` (e.g. with `ensureComposeOrScaffold()` and `newScaffoldCmd()`) actually correct?**
  _`Detect()` has 17 INFERRED edges - model-reasoned connections that need verification._
- **What connects `name`, `version`, `minAppVersion` to the rest of the system?**
  _494 weakly-connected nodes found - possible documentation gaps or missing edges._