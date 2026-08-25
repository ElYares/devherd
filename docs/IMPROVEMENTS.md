# DevHerd — Revisión de Arquitectura e Infraestructura

> Revisión técnica (senior staff) del repositorio `github.com/devherd/devherd`.
> Alcance: arquitectura, infraestructura, calidad/testing, UX/DX, seguridad y roadmap priorizado.
> Esta revisión es de solo lectura: no se modificó código fuente, únicamente se generó este documento.

> **Nota de vigencia.** El diagnóstico original es de **2026-06-19**. Buena parte de sus
> hallazgos ya se resolvieron: ver ["Estado de implementación"](#estado-de-implementacion-2026-07-21)
> y el roadmap al final, que llevan el estado real. En concreto, los titulares 1, 2, 3 y 4
> del resumen ejecutivo **ya están corregidos**: el binario no está en git, existen
> `Makefile`, CI y linter, y hay logging estructurado con `slog`.
>
> Para el estado actual completo del sistema, con métricas medidas, ver
> [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md).

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

### Cobertura (medida con `go test ./... -cover`)

Dos columnas: el diagnóstico original de 2026-06-19 y la medición actual sobre `76a8a24`.
La cobertura global es hoy del **41.3%**.

| Paquete | 2026-06-19 | Actual | Nota |
|---|---|---|---|
| `internal/runner` | — | **100.0%** | Paquete nuevo (item #7) |
| `internal/version` | 0% | **100.0%** | |
| `internal/scaffold` | — | **90.5%** | Paquete nuevo |
| `internal/detector` | 76.1% | 76.1% | |
| `internal/database` | 64.7% | 68.3% | |
| `internal/services` | 12.1% | **67.4%** | Mejorado por el `Runner` inyectable |
| `internal/observe` | 55.7% | 54.7% | |
| `internal/compose` | 50.0% | 51.6% | |
| `internal/proxy` | 40.7% | 48.2% | |
| `internal/dns` | 31.0% | 44.1% | |
| `internal/config` | 40.3% | 40.3% | `Store` (Load/Save) sigue al 0% |
| `internal/doctor` | 19.4% | 39.2% | Mejorado por el seam de exec |
| `internal/preflight` | 17.8% | 38.4% | `inspectPorts` e `inspectLaravelEnv` siguen al 0% |
| `internal/cli` | 5.0% | **6.0%** | Sigue siendo el mayor punto ciego: 2722 LOC |
| `cmd/devherd` | 0% | 0% | Entrypoint |

La cobertura baja se concentra en los paquetes que tocan host/Docker. La adopción del
`Runner` (item #7) mejoró claramente `services`, `doctor`, `preflight`, `dns` y `proxy`,
pero **`internal/cli` sigue prácticamente sin probar**, y es donde vive la orquestación
real.

### Problemas concretos

**1. Funcionalidad anunciada como no implementada (deuda de producto).** *(Parcialmente
resuelto: `logs` ya está implementado; `sentry` sigue siendo un placeholder.)*
`grep notImplemented` → `cli/logs.go:11`, `cli/sentry.go:52` (apply), `:71` (set-dsn), `:87` (test). El README (líneas 21-25) y `root.go:38` registran `logs` y `sentry` como comandos de primera clase. Un usuario que ejecute `devherd logs <proj>` recibe `"logs is not implemented yet"`. 

> Recomendación: ocultar comandos no implementados con `cmd.Hidden = true` o marcarlos `[experimental]` en el `Short`, y alinear el README. Implementar `logs` debería ser sencillo: ya existe `compose.Command(project)` para construir el `docker compose ... logs -f`.

**2. Errores ignorados (`errcheck` los detectaría).**
Ya listados en Arquitectura punto 2 (`observe/server.go:137,146,183`). Un `golangci-lint` con `errcheck` habilitado los marcaría. Recomiendo habilitar al menos: `errcheck`, `govet`, `staticcheck`, `ineffassign`, `gocritic`, `revive`.

**3. Duplicación de helpers.**
`runCommand`/`run`/`runDocker` y `firstLine` aparecen casi idénticos en `proxy/external.go`, `compose/project.go`, `services/manager.go`, `observe/docker.go`. `primaryLabel` (`external.go:291`) y `composeProjectLabel` (`project.go:367`) son funciones casi gemelas. Consolidar en un paquete `internal/exec` y `internal/slug`.

**4. Compose embebido como string literal de Go.** *(Resuelto: ver roadmap #11.)*
`services/manager.go` definía el `docker-compose.yml` de servicios compartidos como una constante string: difícil de leer y de versionar. Hoy se embebe desde un `.yml` real (`//go:embed shared-services.compose.yml`), en línea con el uso de `//go:embed` para las migraciones (`database/migrations.go`) y para `Caddyfile.tmpl` (en `templates/`), de modo que editores y linters de YAML lo validan.

**5. Migraciones que no son migraciones.** *(Resuelto: ver roadmap #16.)*
`database/db.go` ejecutaba el esquema completo en cada arranque, sin versionado incremental: cualquier cambio de columna en una DB existente obligaba a borrar el archivo. Hoy `Manager.Ensure` llama a `migrate()`, que embebe `migrations/*.sql`, aplica solo las migraciones pendientes y las registra en la tabla `schema_migrations` (con `migrations/0001_init.sql` como base y un test de compatibilidad para bases legacy).

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

## Estado de implementacion (2026-07-21)

Verificado sobre el commit `76a8a24`. Cambios respecto a la revision anterior:

**Confirmado como resuelto**

- ✅ **#1** El binario **no** esta en git (`git ls-files bin/` vacio; `bin/` ignorado).
  El hallazgo original del resumen ejecutivo ya no aplica.
- ✅ **#7** `internal/runner` existe con cobertura 100% y esta adoptado en `compose` y
  `services`.
- ✅ **#16** Migraciones versionadas activas, con test de compatibilidad para bases legacy.
- ✅ **#17** Apagado limpio del poller con `WaitGroup`.

**Nuevo desde entonces**

- Comando `devherd scaffold` (genera compose y manifiesto para repos sin contenedores),
  con soporte completo de Laravel. Cobertura 90.5%.
- Comando `devherd serve` (up + proxy apply + open) via `runSiblingCommand`.

**Revisado a la baja**

- 🔶 **#7** La unificacion esta **a medias**: `doctor`, `preflight`, `proxy`, `dns`,
  `observe` y `compose/logs` mantienen su propio helper de `exec`, con timeouts
  inconsistentes (3 s, 5 s, 10 s o ninguno). Es exactamente la duplicacion que el doc del
  paquete afirma haber eliminado.
- 🔶 **#8** `cli` sigue en 6.0%. Sin poder inyectar `appContext`, la capa de comandos no es
  testeable end-to-end. Es hoy el mayor riesgo de calidad del proyecto.

**Deuda nueva detectada**

Ver [SYSTEM-OVERVIEW.md, seccion 10](SYSTEM-OVERVIEW.md#10-riesgos-y-deuda-priorizados)
para el detalle y la priorizacion. Los titulares:

- El collector de Observe **no descomprime envelopes gzip**, lo que impide usar SDKs Sentry
  oficiales sin desactivar la compresion.
- El Caddyfile del proxy externo **se escribe antes de validarse**, sin rollback.
- `prepareComposeProject` traga un error y devuelve el proyecto **sin los overrides**,
  divergiendo en silencio de lo que `proxy apply` espera.
- `observe cleanup` no purga la tabla `containers`, que crece sin limite.
- GoReleaser esta configurado pero **ningun workflow lo dispara** y no hay tags git: el
  release nunca se ha ejecutado.
- Cuatro paquetes fantasma (`api`, `logs`, `runtimes`, `sentry`), tres plantillas no
  referenciadas y cinco tablas SQLite sin uso.

---

## Hallazgos de campo: Observe en un proyecto real (2026-07-22)

Detectados al integrar Observe con un Laravel 13 sobre Docker (`tl-mas-server`), no por
lectura de codigo. Ninguno aparece hoy en `docs/observe.md`.

### Bloqueantes: la ingesta desde contenedores no funciona de fabrica

**O1. `attach` generaba un DSN que ningun contenedor podia usar.** *(Resuelto.)*
`observe attach` construia el DSN con el `--addr` por defecto, `127.0.0.1:9777`. Dentro de
un contenedor esa direccion es el propio contenedor, no el host, asi que el `SENTRY_DSN`
inyectado en el override era inservible **en el unico escenario para el que se diseno**.
Peor: el collector tambien escuchaba solo en loopback, de modo que ni corrigiendo el DSN a
mano se alcanzaba.
> Corregido resolviendo el gateway IPv4 de la red compartida (`observe.InspectNetwork`) y
> usandolo por defecto en los dos lados: `observe start` escucha ahora **a la vez** en
> loopback y en el gateway (`Server.ListenAndServeOn`, multi-listener; si el gateway no
> existe arranca igual en loopback y lo registra), y `attach`/`dsn` construyen el DSN con el
> gateway. Con `--addr` explicito manda el usuario, y cualquier DSN loopback dispara un aviso
> por stderr. El panel sigue sirviendose en `127.0.0.1:9777`, sin exponer nada a la LAN.

**O2. Requisito de firewall no documentado.** *(Resuelto.)*
Con `ufw` activo (default en muchas distros), el trafico contenedor -> host se descarta en
`INPUT`. Los puertos publicados por Docker funcionan porque sus reglas DNAT preceden a las
cadenas de ufw, pero el collector es un listener normal del host y queda bloqueado. Hace
falta una regla explicita del tipo
`ufw allow from 172.18.0.0/16 to 172.18.0.1 port 9777 proto tcp`.
> Corregido con una sonda real en `observe status`: lanza un contenedor efimero en la red
> compartida y pide `/health` desde dentro, que es donde el fallo se manifiesta (el host
> siempre se alcanza a si mismo). Usa la primera imagen ya presente en local —`busybox`,
> `alpine`, `caddy:2-alpine`— y no descarga ninguna: si no hay, sugiere el comando
> equivalente. Al fallar imprime la regla concreta, detectando si ufw esta activo por
> `/etc/ufw/ufw.conf` (`ufw status` exigiria root). Se desactiva con
> `--check-reachability=false`.

**O10. La red del proxy no es la red por la que reporta el proyecto.** *(Resuelto.)*
El arreglo de O1 asumia que `infra_web` servia para todos. Falso, y medido sobre dos
proyectos reales: a la red del proxy solo se conecta el servicio que **publica** el proxy.

| Proyecto | Red propia | `infra_net` | `infra_web` |
|---|---|---|---|
| aang-server | 6/6 contenedores | 2/6 (`app`, `queue`) | 1/6 (`web`) |
| tl-mas-server | 4/4 contenedores | — | 1/4 (`app`) |

Consecuencias: el DSN de `aang-server` apuntaba a una IP que su `app` no alcanzaba, y —peor—
la sonda de O2 respondia **`ok`** porque lanzaba su contenedor efimero en `infra_web`, la
unica red donde todo funcionaba. Un falso positivo en la comprobacion escrita justo para
evitar ese fallo.
> Corregido contando cobertura por red (`ProjectNetworkCoverage`) en vez de asumir una:
> `attach`/`dsn` eligen la red DevHerd que mas contenedores del proyecto cubre, `start`
> escucha en los gateways de todas las redes relevantes —incluidas las de los contenedores
> ya observados— y `observe status <proyecto>` lanza la sonda en la red de ese proyecto.
> Se prefiere una red estable aunque cubra menos: la red privada del proyecto cubre mas, pero
> Docker le asigna otra subred al recrearla y dejaria el DSN inyectado apuntando al vacio.

### Alertas: sin silenciamiento, ruido garantizado

**O3. `new-issue` notifica por cada issue nuevo.** *(Resuelto.)*
Un mismo bug con mensajes variables (`user 42`, `user 43`, ...) generaba un issue —y por
tanto una alerta— por cada variante.
> El enmascarado del fingerprint (correos, UUIDs, hashes y numeros) elimina la causa mas
> comun, y un `fingerprint` explicito da control fino.
> **Correccion al registro anterior:** este documento decia que un issue que reaparece
> seguia sin silenciarse. Es falso: el corte `if !issueWasNew { continue }` ya estaba en el
> codigo y nunca realertaba sobre un issue conocido. Lo que quedaba sin tope era una rafaga
> de issues genuinamente nuevos —un despliegue que rompe 20 cosas distintas—, y eso lo
> cierra el cooldown por regla, 15 minutos por defecto para este tipo.

**O4. `error-rate` notifica en cada evento posterior al umbral, no una vez por ventana.**
*(Resuelto.)*
Verificado: con umbral 3 y ventana 5m, el 3.er evento y **todos** los siguientes dentro de
la ventana producen entrega. 50 errores en 5 minutos = 48 entregas.
> Resuelto con `cooldown_seconds` por regla, evaluado en `insertAlertDelivery`, de modo que
> el silencio vale para los cuatro tipos con un solo mecanismo. El umbral se sigue mirando en
> cada evento; lo que se corta es la entrega repetida. Medido en pruebas: los mismos 50
> eventos pasan de 48 entregas a 1, y con `--cooldown 0` vuelven a ser 48.
> Se descarto la entrega agregada por ventana: dejaria de ser inmediata y necesitaria un
> disparador periodico que hoy no existe.

### Captura de logs: de una foto a dos pasadas

**O5. Los logs se capturaban una unica vez, en la ingesta, y nunca se rellenaban.**
*(Resuelto.)*
`correlation.go:48` ejecutaba `docker logs --since t-30s --until t+30s --tail 200` en el
momento de recibir el evento. Consecuencias: la mitad futura de la ventana estaba casi
siempre vacia (esos logs aun no existen); si no hubo salida en la ventana, el timeline
quedaba vacio para siempre; y un contenedor ruidoso agotaba las 200 lineas con ruido.
> Resuelto con una segunda pasada enganchada al poller de 10 s. Cada tick busca eventos
> cuya ventana futura ya vencio y siguen sin rellenar, y pide solo la mitad que faltaba
> (`--since t --until t+30s`). El estado vive en `events.logs_backfilled`, de modo que
> cada evento se intenta una sola vez y no una por tick.
> Se eligio el poller sobre el relleno perezoso al leer el timeline: capturar cerca del
> evento encuentra al contenedor vivo y sus logs sin rotar, mientras que una consulta
> horas despues se quedaria sin nada igual. El costo es que depende del collector
> encendido —la misma dependencia que ya tiene la ingesta, ver **O7**—, y por eso un
> evento con mas de 5 minutos sin rellenar se da por perdido en vez de reintentarse.
> Efectos colaterales: cada pasada trae su propio tope de 200 lineas, asi que el ruido
> ya no se come el presupuesto de lo que paso *despues* del error; las lineas que las
> dos pasadas ven en comun se insertan una sola vez; y `Timeline` pasa a ordenar por
> marca de tiempo en vez de por orden de insercion, que con dos pasadas dejo de ser lo
> mismo.

**O6. El poller de 10 s puede perder reinicios rapidos.**
`server.go:184` toma instantaneas cada 10 s. Un contenedor que cae y vuelve dentro de ese
intervalo no genera `container_events`, y por tanto no dispara `container-exit`.

### Perdida de datos y cosmetica

**O7. Sin collector no hay evento.** La ingesta es push sincrono sin cola ni reintento: los
errores ocurridos mientras el collector esta apagado se pierden sin rastro. Dado que el
collector es un proceso en foreground que el usuario debe recordar levantar, es un modo de
fallo probable, no teorico.

**O8. Los IDs de issue tienen huecos.** El ID se reserva antes de resolver si el
fingerprint ya existia, de modo que los duplicados consumen numeracion. Cosmetico.

**O9. `events.raw_payload` era una columna de solo escritura.** *(Resuelto.)*
`ListEvents` y `Timeline` seleccionaban 15 de las 16 columnas de `events`, omitiendo
justo `raw_payload`. Todo lo que un SDK envia mas alla del modelo normalizado —`context`,
`tags`, breadcrumbs, stack frames— se guardaba y quedaba inaccesible salvo por SQLite
directo. Corregido exponiendo la columna en ambas consultas, con un helper
`observe.ExtraPayload` que descarta las claves que ya tienen columna propia, un bloque
`Payload:` en `observe timeline` y un campo `payload_extra` en la API del panel. Sin
cambio de esquema: la columna ya existia y ya se poblaba.
> Nota: esto multiplica el valor de arreglar la ingesta de envelopes (ver Resumen
> ejecutivo). Mientras el payload crudo era invisible, casi todo lo que manda un SDK real
> se descartaba de cara al usuario.

---

## Estado de implementacion (2026-08-11)

Verificado sobre el commit `f5d3b3b`.

**El linter nunca habia corrido**

Los items #3 y #4 estaban marcados como hechos, y lo estaban en el sentido de que
los archivos existian. Pero el job `lint` fallaba en **todas** las corridas desde al
menos el 2026-07-22: los PR #4, #5, #6 y #7 se mergearon con el semaforo en rojo.

    can't load config: the Go language version (go1.24) used to build
    golangci-lint is lower than the targeted Go version (1.25.0)

Moria en 90 ms cargando la configuracion, sin analizar una sola linea. Dos causas
encadenadas: `golangci-lint-action@v6` tope en la serie v1 de golangci-lint y
resolvia `version: latest` a v1.64.8 (compilada con go1.24), incompatible con un
`go.mod` en `go 1.25.0`; y `.golangci.yml` era un hibrido que declaraba
`version: "2"` pero conservaba la estructura v1, de modo que `golangci-lint migrate`
lo daba por migrado y v2 lo rechazaba.

Corregido subiendo a `golangci-lint-action@v8` con la version **fijada** en
`v2.12.2` —no `latest`, para que no vuelva a romperse solo— y migrando la config a
mano.

**La primera corrida real: 438 hallazgos**

| Categoria | # | Resolucion |
|---|---|---|
| errcheck en `fmt.Fprint*` | 205 | Excluido: el error solo puede venir de una salida rota, y para entonces no queda donde reportarlo |
| revive `exported` + `package-comments` | 158 | Excluido: comentarios de doc que ahogarian los hallazgos que importan |
| errcheck en `Close`/`Rollback` | 52 | Arreglado con descarte explicito |
| revive `unused-parameter` | 17 | Arreglado |
| staticcheck | 3 | Arreglado (`nil` como Context x2, ley de De Morgan) |
| revive: blank-imports, redefines-builtin-id | 3 | Arreglado |

Las dos exclusiones van comentadas en `.golangci.yml` con su razon, y la lista de
reglas de revive quedo explicita para poder reactivar `exported` y
`package-comments` cuando la documentacion de paquetes se aborde a proposito.

**Leccion**: un job de CI que falla siempre deja de leerse. Conviene revisar que
`lint` este en verde en `main`, no solo que exista.

---

## Roadmap de mejoras priorizado

> Estado: ✅ hecho · 🔶 parcial · (sin marca) pendiente. Ver "Estado de implementacion".

| # | Mejora | Prioridad | Esfuerzo | Notas |
|---|---|---|---|---|
| 1 | ✅ Quitar binario `devherd` de git + `.gitignore /devherd` | **Alta** | XS (min) | **Hecho.** `git rm --cached devherd`; opcional purgar historial |
| 2 | ✅ Añadir `Makefile` con build/test/lint/cover + wiring de `ldflags` a `version.go` | **Alta** | S | **Hecho.** Incluye `version.Long()`; desbloquea release y versionado real |
| 3 | ✅ CI con GitHub Actions (vet + test -race + golangci-lint) | **Alta** | S | **Hecho** (`.github/workflows/ci.yml`). El job `lint` estuvo en rojo desde su creacion hasta el 2026-08-11 por incompatibilidad de versiones; ver "Estado de implementacion (2026-08-11)" |
| 4 | ✅ Configurar `.golangci.yml` (errcheck, staticcheck, gocritic, revive) | **Alta** | S | **Hecho**, pero no corrio hasta el 2026-08-11: el archivo era un hibrido v1/v2. Config migrada y hallazgos saldados |
| 5 | ✅ Alinear README con la realidad y ocultar/etiquetar comandos `notImplemented` | **Alta** | S | **Hecho:** README alineado (logs/serve/flags); `sentry set-dsn`/`test` `Hidden` |
| 6 | 🔶 Introducir `slog` global + flags `--verbose`/`--json`; reemplazar `fmt` de diagnóstico | **Alta** | M | **Parcial:** infra slog + flags `--verbose`/`--log-json` + errores de observe hechos. Falta migrar `fmt` de diagnostico en `cli` y `ConnectProject` |
| 7 | ✅ Extraer interfaz `Runner`/`Executor` y unificar `runCommand/run/runDocker` | **Media** | M | **Hecho:** `internal/runner` en `services`/`compose`; seam de exec en `doctor` y `proxy` (semántica propia preservada) |
| 8 | 🔶 Subir cobertura de `cli` (5%), `services` (12%), `preflight` (18%), `doctor` (19%) tras #7 | **Media** | M | **Parcial:** services 12→67%, doctor 19→39%, preflight 18→38%, dns 31→44%, proxy 40→48%. `cli` aún ~6% (necesita inyectar appContext) |
| 9 | ✅ Implementar `devherd logs` (ya hay `compose.Command`) | **Media** | S | **Hecho** (`internal/compose/logs.go`): flags `-f/--follow` y `--tail`, streaming sin buffer |
| 10 | ✅ Marcadores explícitos en Caddyfile administrado (en vez de conteo de llaves) | **Media** | S | **Hecho:** `# devherd managed start/end`; strip de 2 pasadas (migra formato viejo) |
| 11 | ✅ Embeber compose de servicios compartidos desde `.yml` real (`//go:embed`) | **Media** | XS | **Hecho** (`shared-services.compose.yml`) |
| 12 | ✅ Mensaje proactivo antes de operaciones `sudo` + validación regex de dominios | **Media** | S | **Hecho** (`dns.validateDomains` + aviso por stderr) |
| 13 | ✅ `Dockerfile` (build multi-stage) + `Example:` en comandos Cobra | **Media** | M | **Hecho:** Dockerfile distroless (en `chore/repo-hygiene`) + `Example:` en serve/logs/up/init/proxy |
| 14 | ✅ Comando compuesto `serve` (up + proxy apply + open) | **Baja** | S | **Hecho** (`internal/cli/serve.go`, reusa comandos hermanos) |
| 15 | ✅ GoReleaser + paquete `.deb` (Ubuntu-first) | **Baja** | M | **Hecho:** `.goreleaser.yml` (en `chore/repo-hygiene`) |
| 16 | ✅ Migraciones versionadas (tabla `schema_migrations`) | **Baja** | M | **Hecho** (`migrations/0001_init.sql` + runner idempotente) |
| 17 | ✅ `WaitGroup` para apagado limpio del poller en `observe.Server` | **Baja** | XS | **Hecho** (poller en contexto cancelable + `wg.Wait()` en shutdown) |
| 18 | Implementar `sentry set-dsn`/`test` con manejo seguro de secretos (`0o600`) | **Baja** | M | Depende de roadmap de producto |

**Leyenda de esfuerzo:** XS < 1h · S ≈ medio día · M ≈ 1-3 días.

### Orden sugerido de ataque

1. **Higiene de repo y build (items 1-5)**: una tarde. Quita el binario, mete Makefile + CI + linter, alinea docs. Impacto inmediato y barato.
2. **Logging y testabilidad (6-8)**: la inversión arquitectónica de mayor retorno; el `Runner` desbloquea cobertura real en los paquetes hoy intestables.
3. **Robustez y UX (9-14)**: completar `logs`, endurecer proxy/dns, mejorar ergonomía.
4. **Distribución y madurez (15-18)**: release reproducible, migraciones, secretos.
