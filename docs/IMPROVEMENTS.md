# DevHerd — Revisión de Arquitectura e Infraestructura

> Revisión técnica (senior staff) del repositorio `github.com/devherd/devherd`.
> Alcance: arquitectura, infraestructura, calidad/testing, UX/DX, seguridad y roadmap priorizado.
> Esta revisión es de solo lectura: no se modificó código fuente, únicamente se generó este documento.

---

## Resumen ejecutivo

DevHerd es una CLI escrita en Go (Cobra) que actúa como una plataforma de desarrollo local "Ubuntu-first", inspirada en Herd. Orquesta proyectos basados en `docker compose`, gestiona dominios locales (`.test` / `.localhost`), un proxy reverso Caddy (en host o en contenedor externo administrado), servicios compartidos (Redis, Mailpit), y un subsistema de observabilidad local (`observe`) con servidor HTTP, panel web, ingestión de eventos tipo Sentry y correlación con logs de Docker.

El proyecto está **bien estructurado a nivel de paquetes** (`internal/` con responsabilidades claras), tiene **una base de tests razonable en los paquetes núcleo** (compose, database, detector, observe, proxy) y un diseño de comandos consistente. Es un MVP funcional y coherente.

Hallazgos principales (titulares):

1. **Binario compilado de 21 MB commiteado al repo** (`./devherd`, rastreado en git). Es el problema más visible y de mayor impacto inmediato.
2. **No existe infraestructura de build/CI**: no hay `Makefile`, `Dockerfile`, workflow de GitHub Actions, ni configuración de linter (`.golangci.yml`). El "build" vive en un script shell que hace `go build` directo.
3. **Sin logging estructurado ni observabilidad de la propia CLI**: 155 llamadas `fmt.Print*`/`fmt.Fprint*` en `internal/`, cero uso de `log/slog`. No hay niveles de log, ni flag `--verbose`/`--json`.
4. **Comandos anunciados pero no implementados**: `logs`, `sentry set-dsn`, `sentry test` y el modo "apply" de `sentry init` devuelven `notImplemented`. El README los lista como parte de la superficie pública.
5. **Patrones de manejo de errores que tragan fallos silenciosamente** en rutas críticas (proxy y observe).
6. **Desfase de versión de Go** entre `go.mod` (`go 1.25.0`) y el toolchain instalado (`go 1.26.3`), y `version.go` con metadatos hardcodeados sin wiring de `ldflags`.
7. **Cobertura de tests muy desigual**: paquetes con efectos colaterales sobre el host (`services` 12%, `preflight` 18%, `doctor` 19%, `dns` 31%, `cli` 5%) están poco cubiertos.

A nivel arquitectura el código es sólido; el déficit real es de **infraestructura, disciplina de release y madurez operacional**.

---

## Arquitectura

### Layout actual de paquetes y flujo de datos

```
cmd/devherd/main.go        → entrypoint: cli.Execute() y os.Exit(1) en error
internal/cli/              → capa de comandos Cobra (root.go registra todo)
  app_context.go           → loadAppContext(): paths XDG + config + DB SQLite
internal/config/           → Config (JSON), Paths (XDG), Store load/save atómico
internal/database/         → Manager SQLite (modernc.org/sqlite, sin CGO), projects.go
internal/compose/          → ResolveProject (manifest .devherd.yml o autodetect), Up/Down/Stop
internal/proxy/            → Caddy host (caddy.go) + Caddy docker externo (external.go) + bootstrap
internal/dns/              → SyncHosts: edita /etc/hosts vía sudo
internal/services/         → Manager de stack compartido (redis, mailpit) embebido como string
internal/detector/         → detección de stack/framework de un proyecto
internal/preflight/        → inspección de colisiones antes de `up`
internal/doctor/           → validación de prerequisitos del host
internal/observe/          → server HTTP + store SQLite + correlación Docker + panel
internal/version/          → metadatos de versión
```

Flujo típico (`devherd up <path>`): `cli/up.go` → `loadAppContext` (resuelve paths, carga config, garantiza DB) → `preflight.Inspect` → `prepareComposeProject` → `compose.UpProject` → `exec docker compose up --build -d`. El proxy (`proxy apply`) construye `ExternalProject`, escribe overrides de compose, conecta a la red `infra_web`, regenera el `Caddyfile` y hace `caddy reload` dentro del contenedor.

### Fortalezas

- **Separación de capas clara**: la capa CLI (`internal/cli`) no contiene lógica de negocio pesada; delega en paquetes de dominio. `app_context.go:20-55` centraliza la inicialización (paths, config, DB) de forma reutilizable.
- **`compose.Project` como modelo de dominio**: `ResolveProject` (`internal/compose/project.go:63`) unifica manifest y autodetección, y `ProjectNameForPath` (`:344`) genera un nombre estable por hash SHA1 de la ruta absoluta — buena decisión para aislar clones homónimos.
- **Escritura atómica de config**: `config.Store.Save` (`internal/config/config.go:119-135`) escribe a `.tmp` y hace `os.Rename`, con permisos `0o600`. Correcto.
- **Abstracción `DockerRuntime`** en observe (`internal/observe/docker.go:14`): es la **única interfaz real de inyección de dependencias** del proyecto y permite tests con un fake (`NewServerWithDocker`). Es el patrón a replicar en el resto del código.
- **Timeouts en exec**: `proxy/external.go:472` y `observe/docker.go:118` usan `context.WithTimeout` alrededor de los comandos `docker`.

### Problemas y recomendaciones

**1. Ausencia generalizada de interfaces / acoplamiento a `os/exec` y al filesystem.**
Salvo `DockerRuntime`, casi todos los paquetes llaman directamente a `exec.Command("docker", ...)`, `os.ReadFile`/`os.WriteFile` y rutas absolutas como `/etc/hosts`. Ejemplos: `compose.run` (`project.go:308`), `services.runDocker` (`manager.go:154`), `proxy.runCommand` (`external.go:472`), `dns.SyncHosts` (`hosts.go:17`). Esto hace que la lógica solo sea testeable end-to-end con Docker real, lo que explica la baja cobertura de `services` (12%) y `preflight` (18%).

> Recomendación: extraer una interfaz `CommandRunner` (o `Executor`) compartida, p. ej.:
> ```go
> type Runner interface {
>     Run(ctx context.Context, dir, name string, args ...string) (string, error)
> }
> ```
> Inyectarla en `compose`, `services`, `proxy` y `doctor`. Esto permite tests de tabla con un fake que devuelve salidas predefinidas, sin Docker. Hoy `runCommand`/`run`/`runDocker` están **triplicados casi idénticos** en tres paquetes (`external.go:472`, `project.go:308`, `services.go:154`) — unificar elimina duplicación y dispersión de comportamiento (timeouts distintos: 10s en proxy, 5s en observe, ninguno en compose/services).

**2. Manejo de errores que traga fallos silenciosamente en rutas críticas.**
- `proxy/external.go:176-181` (`ConnectProject`): si `composeServiceContainer` falla, hace `continue` sin registrar nada. Un servicio que no arrancó deja el proxy a medias y el usuario no se entera.
- `observe/server.go:137` `logs, _ = correlator.CorrelateEvent(...)`, `:146` `_ = s.store.StoreContainerLogs(...)`, `:183` `_, _ = s.store.StoreContainers(...)`: tres errores descartados en el path de ingestión. Si la persistencia de logs/containers falla, no hay ningún rastro.
- `observe/server.go:179` (`snapshotObservedContainers`): `if err != nil || len(containers) == 0 { return }` — colapsa "error" y "sin datos" en el mismo branch silencioso dentro de un loop que corre cada 10s.

> Recomendación: introducir `slog` y registrar estos errores a nivel `WARN`/`ERROR` con contexto (proyecto, servicio, container). Para `ConnectProject`, decidir explícitamente si un servicio caído debe abortar o degradar, y comunicarlo al usuario.

**3. Concurrencia simple pero con apagado incompleto.**
`observe/server.go:62-82`: `ListenAndServe` lanza el server y el poller en goroutines. El poller (`pollObservedContainers`) respeta `ctx.Done()`, pero **no hay `sync.WaitGroup`**: en el shutdown, `server.Shutdown` se ejecuta pero no se espera a que la goroutine del poller termine de drenar su iteración en curso (que puede tener un `exec docker` de hasta 5s en vuelo). Es benigno hoy pero conviene cerrar el ciclo de vida correctamente con un `WaitGroup` y propagar el `ctx` del request hacia el poller.

**4. Parsing de Caddyfile basado en strings.**
`stripManagedDomains` (`external.go:371-407`) y `renderExternalSite` (`:346`) manipulan el Caddyfile contando llaves `{`/`}` línea a línea. Funciona para el formato que DevHerd genera, pero es frágil ante cualquier edición manual del usuario (comentarios con llaves, llaves en la misma línea, here-strings). 

> Recomendación: encapsular el Caddyfile administrado en marcadores explícitos (`# devherd managed start <domain>` / `# devherd managed end`), igual que ya se hace en `dns/hosts.go` con `# devherd start`/`# devherd end`. Eso evita el conteo de llaves y hace el stripping determinista. Hay una asimetría: el patrón de bloque administrado ya existe para hosts pero no se reutilizó para Caddy.

**5. `main.go` minimalista pero sin códigos de salida diferenciados.**
`cmd/devherd/main.go:11-14` siempre sale con `os.Exit(1)`. No distingue error de usuario (input inválido) de error de sistema (Docker caído). Considerar códigos de salida convencionales (2 para uso incorrecto) para scripting.

**6. Frameworks hardcodeados.**
`proxy/external.go:112-129` y `detector` solo conocen `"vue+flask"` (rutas fijas `backend:8000` / `frontend:5173`). El camino correcto (manifest `.devherd.yml` con `proxy.service`/`proxy.port`) ya existe y es superior; el `switch` por framework debería tratarse como fallback deprecado y documentarse así, para no acumular casos especiales.

---

## Infraestructura

### Estado actual

| Área | Estado |
|---|---|
| Build | Script shell `scripts/install-ubuntu.sh` con `go build` directo. **Sin Makefile.** |
| Dependencias | `go.mod` limpio (Cobra, yaml.v3, modernc sqlite sin CGO). `go.sum` presente. |
| Docker | El proyecto **orquesta** Docker pero **no se distribuye como imagen**. Sin `Dockerfile`. |
| CI/CD | **Inexistente.** No hay `.github/workflows/`. |
| Linter | **Inexistente.** No hay `.golangci.yml`. `go vet ./...` pasa limpio. |
| Observabilidad propia | Solo `fmt.Print*` (155 ocurrencias). Sin `slog`, métricas ni tracing de la CLI. |
| Config | JSON en `~/.config/devherd` (XDG), escritura atómica. Correcto. |
| Secretos | DSN de Sentry previsto pero `set-dsn` no implementado; aún no hay manejo real de secretos. |
| Release/versionado | `version.go` con strings hardcodeados, sin `ldflags`. |

### Problemas concretos

**1. Binario de 21 MB commiteado al repositorio.**
`git ls-files` confirma que `./devherd` (21 MB) está **rastreado en git**. Esto infla el clone, contamina el historial y queda obsoleto con cada commit de código. El `.gitignore` ignora `bin/` y `dist/` pero **no** el binario en la raíz.

> Acción inmediata:
> ```bash
> git rm --cached devherd
> echo "/devherd" >> .gitignore
> ```
> Para purgarlo del historial (opcional, si pesa el clone): `git filter-repo --path devherd --invert-paths`.

**2. Sin Makefile / automatización de tareas.**
Cada operación (build, test, lint, cover) se ejecuta a mano. Propuesta mínima:

```makefile
.PHONY: build test lint cover install
BIN := bin/devherd
LDFLAGS := -X github.com/devherd/devherd/internal/version.Version=$(shell git describe --tags --always) \
           -X github.com/devherd/devherd/internal/version.Commit=$(shell git rev-parse --short HEAD) \
           -X github.com/devherd/devherd/internal/version.Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:   ; go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/devherd
test:    ; go test ./... -race -count=1
cover:   ; go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out
lint:    ; golangci-lint run
install: ; go install -ldflags "$(LDFLAGS)" ./cmd/devherd
```

Esto además resuelve el wiring de `ldflags` que hoy falta (`version.go` nunca recibe `Commit`/`Date` reales y `version.String()` ni siquiera los expone).

**3. Sin CI.** Workflow mínimo recomendado (`.github/workflows/ci.yml`):

```yaml
name: ci
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - run: go vet ./...
      - run: go test ./... -race -coverprofile=coverage.out
      - uses: golangci/golangci-lint-action@v6
```

Como el binario usa `modernc.org/sqlite` (Go puro, sin CGO), la matriz de CI es trivial — gran ventaja que conviene capitalizar con builds reproducibles vía GoReleaser.

**4. Sin pipeline de release.** Dado que es una CLI distribuida a usuarios, añadir **GoReleaser** (`.goreleaser.yml`) para publicar binarios por plataforma con `ldflags`, checksums y, opcionalmente, paquetes `.deb` (encaja con el foco "Ubuntu-first").

**5. Desfase de toolchain.** `go.mod` declara `go 1.25.0`; el entorno tiene `go 1.26.3`. No es un error, pero conviene fijar `go-version-file: go.mod` en CI y considerar un `toolchain` directive para builds deterministas.

**6. Observabilidad de la propia CLI.** Reemplazar `fmt.Fprint*` de diagnóstico por `slog` con un `--verbose` (sube a `DEBUG`) y `--json` (handler JSON) globales en `root.go`. La salida "de producto" (lo que el usuario debe leer, p. ej. `caddyfile: ...`) se mantiene en stdout; los diagnósticos van por `slog` a stderr. Hoy ambos se mezclan vía `fmt`.

---

## Calidad de código y testing

### Cobertura actual (medida con `go test ./... -cover`)

| Paquete | Cobertura | Nota |
|---|---|---|
| `internal/detector` | 76.1% | Bien |
| `internal/database` | 64.7% | Bien |
| `internal/observe` | 55.7% | Aceptable (gracias a `DockerRuntime` fake) |
| `internal/compose` | 50.0% | Lógica de orquestación sin cubrir |
| `internal/proxy` | 40.7% | Falta cubrir paths con `docker exec` |
| `internal/config` | 40.3% | |
| `internal/dns` | 31.0% | `SyncHosts` (sudo) sin cubrir |
| `internal/doctor` | 19.4% | Mayoría toca el host |
| `internal/preflight` | 17.8% | |
| `internal/services` | 12.1% | Casi todo es `exec docker` |
| `internal/cli` | 5.0% | La capa de comandos casi no se prueba |
| `internal/version` | 0% | Sin tests |
| `cmd/devherd` | 0% | Entrypoint |

20 archivos de test frente a 52 de código (no-test). La cobertura baja se concentra exactamente en los paquetes que tocan host/Docker — síntoma directo de la falta de interfaces (ver Arquitectura, punto 1).

### Problemas concretos

**1. Funcionalidad anunciada como no implementada (deuda de producto).**
`grep notImplemented` → `cli/logs.go:11`, `cli/sentry.go:52` (apply), `:71` (set-dsn), `:87` (test). El README (líneas 21-25) y `root.go:38` registran `logs` y `sentry` como comandos de primera clase. Un usuario que ejecute `devherd logs <proj>` recibe `"logs is not implemented yet"`. 

> Recomendación: ocultar comandos no implementados con `cmd.Hidden = true` o marcarlos `[experimental]` en el `Short`, y alinear el README. Implementar `logs` debería ser sencillo: ya existe `compose.Command(project)` para construir el `docker compose ... logs -f`.

**2. Errores ignorados (`errcheck` los detectaría).**
Ya listados en Arquitectura punto 2 (`observe/server.go:137,146,183`). Un `golangci-lint` con `errcheck` habilitado los marcaría. Recomiendo habilitar al menos: `errcheck`, `govet`, `staticcheck`, `ineffassign`, `gocritic`, `revive`.

**3. Duplicación de helpers.**
`runCommand`/`run`/`runDocker` y `firstLine` aparecen casi idénticos en `proxy/external.go`, `compose/project.go`, `services/manager.go`, `observe/docker.go`. `primaryLabel` (`external.go:291`) y `composeProjectLabel` (`project.go:367`) son funciones casi gemelas. Consolidar en un paquete `internal/exec` y `internal/slug`.

**4. Compose embebido como string literal de Go.**
`services/manager.go:169-210` define el `docker-compose.yml` de servicios compartidos como una constante string. Es difícil de leer y versionar. Dado que el proyecto **ya usa `//go:embed`** para `schema.sql` (`database/migrations.go`) y para `Caddyfile.tmpl` (en `templates/`), debería embeberse desde un archivo `.yml` real para consistencia y para que editores/linters de YAML lo validen.

**5. Migraciones que no son migraciones.**
`database/db.go:40` ejecuta `schemaSQL` completo en cada arranque (idempotente vía `CREATE TABLE IF NOT EXISTS`, presumiblemente). El archivo `migrations.go` solo hace `//go:embed schema.sql`. No hay versionado incremental: cualquier cambio de columna en una DB existente requiere borrar el archivo. Para un MVP es aceptable, pero conviene un esquema de migraciones versionadas (tabla `schema_migrations` + archivos numerados) antes de tener usuarios con datos.

**6. Idiomático: salidas de usuario vía `cmd.OutOrStdout()` (bien), pero diagnósticos vía `fmt` (mal).** Ver Infraestructura punto 6.

---

## Formas de usar el proyecto (UX/DX)

### Cómo se usa hoy

Durante desarrollo: `go run ./cmd/devherd <cmd>`. Instalación: `scripts/install-ubuntu.sh` compila a `~/.local/bin/devherd`; `scripts/install-caddy-ubuntu.sh` instala Caddy vía apt. Flujo típico documentado en README: `init → doctor → park → plan → inspect → domain set → up → proxy apply → open → list`.

### Fricciones detectadas

1. **El README enseña rutas absolutas inexistentes** (`/home/elyarestark/develop/examples/...`) y mezcla `elyares`/`elyarestark`. Para onboarding es confuso; usar rutas relativas o `$(pwd)`.
2. **Dos pasos manuales tras `up`** (`up` y luego `proxy apply`). Un comando `devherd serve <path>` que encadene `up` + `proxy apply` + `open` reduciría fricción.
3. **Comandos que prometen y fallan** (`logs`, `sentry set-dsn/test`). Mala primera impresión.
4. **`sudo` opaco**: `dns.SyncHosts` (`hosts.go:39-44`) invoca `sudo -v` y `sudo cp` de forma interactiva sin anunciar previamente que va a tocar `/etc/hosts`. En modo `caddy-docker-external` (TLD `.localhost`) esto ni siquiera hace falta. Conviene un mensaje explícito ("DevHerd necesita sudo para actualizar /etc/hosts con N dominios") y un `--dry-run` global.
5. **Sin autocompletado documentado**: Cobra lo soporta gratis (`devherd completion bash|zsh|fish`); no está mencionado en docs.
6. **Sin `--help` enriquecido por comando con ejemplos** (`cobra.Command.Example`).

### Mejoras de ergonomía recomendadas

- Comando compuesto `serve`/`start` (up + proxy + open).
- Flags globales: `--verbose`, `--json`, `--dry-run`, `--config <path>`.
- `Example:` en cada comando + sección "Common workflows" en `--help`.
- Mensaje proactivo antes de cualquier operación con `sudo`.
- Mantener un único set de rutas de ejemplo coherentes en toda la documentación.
- Publicar binarios con GoReleaser para que `install-ubuntu.sh` pueda hacer `curl | install` en vez de exigir toolchain de Go.

---

## Seguridad

Relevante por ser una herramienta que invoca `docker`, edita `/etc/hosts` con `sudo` y levanta un servidor HTTP local.

1. **Inyección de comandos: bajo riesgo, pero verificar.** Todas las invocaciones usan `exec.Command(name, args...)` con argumentos separados (no `sh -c`), lo que evita shell injection. Sin embargo, dominios/aliases derivan de nombres de proyecto y rutas; `primaryLabel`/`composeProjectLabel` ya sanitizan a `[a-z0-9-]`, lo cual es una buena defensa. Mantener esa sanitización como invariante y testearla con inputs adversariales (nombres con `;`, `$()`, unicode).

2. **Edición de `/etc/hosts` con privilegios elevados.** `dns/hosts.go` escribe un temp file y lo copia con `sudo cp`. Riesgos: (a) el temp se crea en el TMPDIR global con permisos por defecto y se elimina con `defer os.Remove` — correcto, pero el contenido viaja por `/tmp`; (b) no hay validación de que los dominios no contengan saltos de línea o entradas maliciosas antes de escribir. Recomendado: validar cada dominio contra una regex estricta antes de construir el bloque (`buildManagedBlock`, `hosts.go:78`).

3. **Servidor de observabilidad.** `observe/server.go` escucha por defecto en `127.0.0.1:9777` (bien, no expuesto) con `ReadHeaderTimeout` y `MaxBytesReader` de 2 MB en el body (bien). Pero **no hay autenticación**: cualquier proceso local puede postear eventos arbitrarios al panel y a la DB. Para un panel local es aceptable, pero documentar el modelo de amenazas y considerar un token local en config si el bind pasara a `0.0.0.0`.

4. **Permisos de archivos.** Config `0o600` (bien). Overrides de compose y Caddyfile se escriben `0o644` (`external.go:162,208`) — aceptables (no contienen secretos hoy), pero el `.env` del proxy externo (cuando se añadan DSN/secretos) debe ser `0o600`.

5. **Sentry DSN como secreto.** Aún no implementado, pero `sentry set-dsn` deberá almacenar el DSN fuera del repo del proyecto y con permisos restringidos; no inyectarlo en archivos versionables (`docker-compose.yml`).

6. **Redes Docker administradas** (`infra_web`, `infra_net`) se crean con labels `devherd.managed=true`; bien para limpieza. No se detectó montaje del socket de Docker ni privilegios de contenedor peligrosos.

---

## Estado de implementacion (2026-06-19)

Parte del roadmap ya se ejecuto. Resumen de lo completado:

**Fase 0 — Higiene de repo y build (items 1-5):**

- ✅ **#1** Binario `devherd` (21 MB) fuera de git; `/devherd` anadido a `.gitignore`. El
  build ahora vive en `make build` → `bin/devherd`.
- ✅ **#2** Nuevo `Makefile` con targets `build`, `install`, `test` (`-race`), `cover`,
  `vet`, `lint`, `tidy`, `run`, `clean`, `help`. Inyecta `version.Version/Commit/Date` via
  `-ldflags`. Se anadio `version.Long()` y `devherd --version` ahora muestra commit y fecha.
- ✅ **#3** CI con GitHub Actions (`.github/workflows/ci.yml`): `go vet` + `make build` +
  `go test -race` + cobertura + `golangci-lint`.
- ✅ **#4** Linter configurado (`.golangci.yml`): `errcheck`, `govet`, `staticcheck`,
  `ineffassign`, `unused`, `gocritic`, `revive`.
- 🔶 **#5** (parcial) `sentry set-dsn` y `sentry test` ahora estan ocultos (`Hidden: true`).
  Falta alinear el README con la realidad (trabajo en curso).

**Fase 1 — Logging y comando `logs`:**

- 🔶 **#6** (parcial) Infraestructura de logging con `slog` lista: flags globales
  `--verbose` (DEBUG) y `--log-json`, diagnosticos a stderr (`internal/cli/logging.go`).
  Los 3 errores antes tragados en `internal/observe/server.go` ahora se loguean con
  `slog.Warn`. **Falta** migrar el resto de `fmt` de diagnostico en `cli` y los fallos
  silenciosos de `ConnectProject` (proxy).
- ✅ **#9** `devherd logs [path]` implementado (antes era stub `notImplemented`): flags
  `-f/--follow` y `--tail N`, con streaming sin buffer (`internal/compose/logs.go`).

El resto de los items siguen pendientes segun la tabla de abajo.

---

## Roadmap de mejoras priorizado

> Estado: ✅ hecho · 🔶 parcial · (sin marca) pendiente. Ver "Estado de implementacion".

| # | Mejora | Prioridad | Esfuerzo | Notas |
|---|---|---|---|---|
| 1 | ✅ Quitar binario `devherd` de git + `.gitignore /devherd` | **Alta** | XS (min) | **Hecho.** `git rm --cached devherd`; opcional purgar historial |
| 2 | ✅ Añadir `Makefile` con build/test/lint/cover + wiring de `ldflags` a `version.go` | **Alta** | S | **Hecho.** Incluye `version.Long()`; desbloquea release y versionado real |
| 3 | ✅ CI con GitHub Actions (vet + test -race + golangci-lint) | **Alta** | S | **Hecho** (`.github/workflows/ci.yml`). Trivial por ser CGO-free (modernc sqlite) |
| 4 | ✅ Configurar `.golangci.yml` (errcheck, staticcheck, gocritic, revive) | **Alta** | S | **Hecho.** Captura los errores ignorados de observe |
| 5 | 🔶 Alinear README con la realidad y ocultar/etiquetar comandos `notImplemented` | **Alta** | S | **Parcial:** `sentry set-dsn`/`test` ya estan `Hidden`. Falta alinear el README |
| 6 | 🔶 Introducir `slog` global + flags `--verbose`/`--json`; reemplazar `fmt` de diagnóstico | **Alta** | M | **Parcial:** infra slog + flags `--verbose`/`--log-json` + errores de observe hechos. Falta migrar `fmt` de diagnostico en `cli` y `ConnectProject` |
| 7 | Extraer interfaz `Runner`/`Executor` y unificar `runCommand/run/runDocker` | **Media** | M | Habilita tests sin Docker; sube cobertura services/proxy/compose |
| 8 | Subir cobertura de `cli` (5%), `services` (12%), `preflight` (18%), `doctor` (19%) tras #7 | **Media** | M | Tests de tabla con fake Runner |
| 9 | ✅ Implementar `devherd logs` (ya hay `compose.Command`) | **Media** | S | **Hecho** (`internal/compose/logs.go`): flags `-f/--follow` y `--tail`, streaming sin buffer |
| 10 | Marcadores explícitos en Caddyfile administrado (en vez de conteo de llaves) | **Media** | S | Robustez ante edición manual; reusar patrón de `dns/hosts` |
| 11 | Embeber compose de servicios compartidos desde `.yml` real (`//go:embed`) | **Media** | XS | Consistencia con `schema.sql`/templates |
| 12 | Mensaje proactivo antes de operaciones `sudo` + validación regex de dominios | **Media** | S | UX + seguridad de `/etc/hosts` |
| 13 | `Dockerfile` (build multi-stage) + `Example:` en comandos Cobra + completion docs | **Media** | M | DX/onboarding |
| 14 | Comando compuesto `serve` (up + proxy apply + open) | **Baja** | S | Reduce fricción del flujo diario |
| 15 | GoReleaser + paquete `.deb` (Ubuntu-first) | **Baja** | M | Distribución sin exigir toolchain Go |
| 16 | Migraciones versionadas (tabla `schema_migrations`) | **Baja** | M | Antes de tener usuarios con datos persistentes |
| 17 | `WaitGroup` para apagado limpio del poller en `observe.Server` | **Baja** | XS | Corrección de ciclo de vida |
| 18 | Implementar `sentry set-dsn`/`test` con manejo seguro de secretos (`0o600`) | **Baja** | M | Depende de roadmap de producto |

**Leyenda de esfuerzo:** XS < 1h · S ≈ medio día · M ≈ 1-3 días.

### Orden sugerido de ataque

1. **Higiene de repo y build (items 1-5)**: una tarde. Quita el binario, mete Makefile + CI + linter, alinea docs. Impacto inmediato y barato.
2. **Logging y testabilidad (6-8)**: la inversión arquitectónica de mayor retorno; el `Runner` desbloquea cobertura real en los paquetes hoy intestables.
3. **Robustez y UX (9-14)**: completar `logs`, endurecer proxy/dns, mejorar ergonomía.
4. **Distribución y madurez (15-18)**: release reproducible, migraciones, secretos.
