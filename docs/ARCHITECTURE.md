# Arquitectura de DevHerd

Esta guia describe la arquitectura interna de DevHerd: el layout de paquetes, las
responsabilidades de cada uno, el flujo de datos a traves del proxy y los tipos y
funciones clave. Todas las referencias apuntan a archivos y lineas reales del
repositorio.

> Toda la documentacion se basa en el codigo actual del repositorio. DevHerd es un
> producto en estado MVP/alpha (`version.Version = "0.1.0-alpha"`, ver
> `internal/version/version.go:7`).

Documentos hermanos: [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md) analiza el estado real del
sistema, sus limitaciones medidas y su deuda tecnica; [USAGE.md](USAGE.md) es la
referencia de comandos.

## 1. Vision general

DevHerd es una plataforma local de desarrollo, pensada como un producto "Ubuntu-first"
inspirado en herramientas como Herd. Es una **CLI escrita en Go con
[Cobra](https://github.com/spf13/cobra)** que orquesta proyectos basados en Docker
Compose, gestiona un proxy reverso local (dominios `.test` / `.localhost`), levanta
servicios compartidos (Redis, Mailpit) y ofrece un collector local de observabilidad
("Observe").

La CLI **no es un daemon**: cada invocacion abre la configuracion, abre la base SQLite,
ejecuta el comando y sale. La unica excepcion es `devherd observe start`, que arranca un
servidor HTTP de larga duracion.

Dependencias principales (`go.mod`):

- `github.com/spf13/cobra` — framework de comandos CLI.
- `modernc.org/sqlite` — driver SQLite en Go puro (sin CGO).
- `gopkg.in/yaml.v3` — parseo de manifiestos `.devherd.yml` y archivos compose.

## 2. Diagrama de componentes

```
                          cmd/devherd/main.go
                                  |
                                  v
                         internal/cli (Cobra)
        +-------------------------+-------------------------+
        |   root.go: registra todos los subcomandos        |
        |   app_context.go: carga config + SQLite (DB)     |
        +--------------------------------------------------+
              |          |           |          |        |
              v          v           v          v        v
        +---------+ +---------+ +----------+ +-------+ +---------+
        | config  | |database | | detector | |compose| |preflight|
        | (XDG    | | (SQLite | | (escanea | |(docker| |(colision|
        |  paths) | | migrado)| | stacks)  | | compose| |  checks)|
        +---------+ +---------+ +----------+ +-------+ +---------+
              |                        |         |        |
              |                        v         v        v
              |                  +----------+ +-----------------------+
              |                  | scaffold | |  runner (seam de exec)|
              |                  | (genera  | +-----------------------+
              |                  |  compose)|
              |                  +----------+
              v                                 |
        +-----------------------------------------------------------+
        | proxy: caddy (host) | caddy-docker-external (local_proxy) |
        +-----------------------------------------------------------+
              |                                  |
              v                                  v
        +---------+                       +-----------+
        |  dns    |                       | services  |
        |/etc/host|                       | redis/mail|
        +---------+                       +-----------+

        +-----------+   +---------+   +----------+
        |  doctor   |   | observe |   |  sentry  |
        | (prereqs) |   |(collector|  |(placehol-|
        |           |   |  + panel)|  |   der)   |
        +-----------+   +---------+   +----------+
```

Flujo tipico de un usuario:

```
init -> park -> [scaffold] -> (plan / inspect) -> up -> proxy apply -> open
                                                   |
                                                   +-> service start redis|mailpit
                                                   +-> observe attach + observe start
```

`devherd serve` (`internal/cli/serve.go:12`) encadena `up` + `proxy apply` + `open`
reutilizando los comandos hermanos via `runSiblingCommand` (`serve.go:57`), sin duplicar
logica.

## 3. Layout de paquetes y responsabilidades

| Paquete | Ruta | Responsabilidad |
|---------|------|-----------------|
| `main` | `cmd/devherd/main.go` | Punto de entrada. Llama a `cli.Execute()` y mapea errores a `os.Exit(1)`. |
| `cli` | `internal/cli/` | Definicion de todos los comandos Cobra, parseo de flags, orquestacion. |
| `config` | `internal/config/` | Resolucion de rutas XDG (`paths.go`) y modelo/persistencia de la config JSON (`config.go`). |
| `database` | `internal/database/` | Gestion de SQLite: esquema, migraciones, CRUD de proyectos y dominios. |
| `detector` | `internal/detector/` | Deteccion de stacks/frameworks de un directorio (Laravel, Node, Vue, Flask, Python, Go, Docker). |
| `compose` | `internal/compose/` | Resolucion de proyectos Compose, manifiesto `.devherd.yml`, ejecucion de `docker compose`. |
| `preflight` | `internal/preflight/` | Inspeccion previa: colisiones de puertos, container_name, volumenes, env de Laravel, redes compartidas, proxy. |
| `scaffold` | `internal/scaffold/` | Generacion de `docker-compose.devherd.yml` y `.devherd.yml` para repos sin contenedores. |
| `proxy` | `internal/proxy/` | Render y aplicacion del proxy reverso: Caddy en host y `caddy-docker-external`. |
| `dns` | `internal/dns/` | Validacion de dominios y sincronizacion del bloque administrado en `/etc/hosts`. |
| `services` | `internal/services/` | Stack compose de servicios compartidos (Redis, Mailpit) en red `infra_net`. |
| `runner` | `internal/runner/` | Seam de ejecucion de comandos externos: interfaz `Runner` + `Cmd` con timeout y trazas `slog`. |
| `doctor` | `internal/doctor/` | Validacion de prerequisitos del host (Docker, Caddy, puertos, redes). |
| `observe` | `internal/observe/` | Collector local de errores, panel web, store SQLite separada, alertas, correlacion Docker. |
| `version` | `internal/version/` | Metadatos de version. `String()` devuelve la version semantica; `Long()` la enriquece con commit y fecha (`Version (commit X, built Y)`), usada por `devherd --version`. Las variables `Version/Commit/Date` se inyectan en build via `-ldflags` (ver `Makefile`). |
| `templates/` | `templates/` | Plantillas del proxy externo, embebidas con `go:embed` desde `templates/proxy-external/embed.go`. |

**Paquetes placeholder**: `internal/api`, `internal/logs`, `internal/runtimes` e
`internal/sentry` contienen unicamente un `doc.go` de dos lineas, sin implementacion. Los
dos ultimos inducen a error: la funcionalidad de logs ya vive en `internal/compose/logs.go`
y la de Sentry en `internal/cli/sentry.go`.

**Plantillas no usadas**: solo `templates/proxy-external/` esta embebida. Los directorios
`templates/caddy/` (duplicado de `internal/proxy/Caddyfile.tmpl`, que es el real),
`templates/nginx/` (no hay driver nginx) y `templates/docker/` (superado por
`internal/services/shared-services.compose.yml`) no los referencia ningun `.go`.

## 4. Punto de entrada y registro de comandos

- `cmd/devherd/main.go:10` — `main()` invoca `cli.Execute()`.
- `internal/cli/root.go:10` — `Execute()` ejecuta `newRootCmd()`.
- `internal/cli/root.go:14` — `newRootCmd()` crea el comando raiz `devherd` y registra los
  18 subcomandos de primer nivel via `cmd.AddCommand(...)` (`root.go:35-52`), en este
  orden: `init`, `doctor`, `park`, `list`, `domain`, `proxy`, `plan`, `inspect`,
  `scaffold`, `up`, `serve`, `stop`, `down`, `open`, `logs`, `service`, `observe`,
  `sentry`.

El root configura `SilenceErrors` y `SilenceUsage` para imprimir errores limpios desde
`main` (`root.go:22-23`).

### Logging de diagnostico (slog)

El root expone dos flags persistentes (`root.go:30-31`): `--verbose` (baja el nivel a DEBUG)
y `--log-json` (handler JSON). En `PersistentPreRunE` llama a `setupLogging`
(`internal/cli/logging.go:18`), que configura el logger global de `log/slog`:

- Los **diagnosticos** (slog) van a **stderr**; con `--log-json` en formato JSON, si no en
  texto. El nivel por defecto es INFO, DEBUG con `--verbose`.
- La salida **"de producto"** (lo que el usuario debe leer) sigue yendo a **stdout** via
  `cmd.OutOrStdout()`.

Esta separacion stdout/stderr permite redirigir cada flujo por separado. El collector de
observe ya usa este logger: `internal/observe/server.go` registra con `slog.Warn` errores
que antes se descartaban silenciosamente en el path de ingestion (correlacion de eventos y
persistencia de logs/containers).

## 5. Contexto de aplicacion (config + base de datos)

La mayoria de comandos comparten el patron `loadAppContext`:

- `internal/cli/app_context.go:15` — el tipo `appContext` agrupa `config.Paths`,
  `config.Config` y `*sql.DB`.
- `internal/cli/app_context.go:21` — `loadAppContext(ctx)`:
  1. Resuelve rutas XDG (`config.ResolvePaths`).
  2. Crea los directorios locales (`paths.Ensure`).
  3. Carga `config.json`; si no existe, devuelve el error
     *"DevHerd is not initialized. Run `devherd init` first"* (`app_context.go:34`).
  4. Aplica defaults de rutas a la config (`cfg.ApplyPathDefaults`).
  5. Garantiza el esquema SQLite (`database.Manager.Ensure`) y abre la conexion.

Los comandos hacen `defer app.DB.Close()` tras cargar el contexto.

### Rutas locales (XDG)

`internal/config/paths.go:20` — `ResolvePaths()` calcula rutas siguiendo el estandar XDG.
En Linux:

- Config: `~/.config/devherd/` -> `config.json` (`paths.go:39,45`).
- Datos: `~/.local/share/devherd/` -> `devherd.db`, `proxy/`, `compose/` (`paths.go:46-51`).
- Estado: `~/.local/state/devherd/` -> `logs/` (`paths.go:41,49`).

`Paths.Ensure()` (`paths.go:55`) crea todos los directorios con permisos `0o755`.

### Configuracion

`internal/config/config.go:9` define `Config`, con subestructuras `ProxyConfig`,
`DNSConfig` y `ObservabilityConfig`. `Default()` (`config.go:38`) entrega los valores
por defecto: driver de proxy `caddy`, TLD `test`, runtime manager `mise`, red externa
`infra_web`, contenedor proxy `infra_caddy`, proveedor de observabilidad `sentry-cloud`.

La config se persiste como JSON con escritura atomica (escribe a `.tmp` y hace `rename`,
`config.go:119-135`), con permisos `0600`.

### Ejecucion de comandos externos (`runner`)

`internal/runner/runner.go` define el seam de ejecucion pensado para unificar la semantica
que antes estaba duplicada en varios paquetes:

- `Runner` (`runner.go:19`) es la interfaz: `Run(ctx, dir, name, args...) (string, error)`.
- `Cmd` (`runner.go:24`) la implementa con timeout opcional, captura **combinada** de
  stdout+stderr, y trazas `slog.Debug` de inicio y fin con el tiempo transcurrido — que es
  lo que hace util el flag global `--verbose`.
- En caso de error, si el proceso escribio algo, **esa salida sustituye al error**
  (`runner.go:50-56`), de modo que el usuario ve el mensaje real de Docker.

Es inyectable: `compose` lo expone como variable de paquete (`project.go:18`) y `services`
como campo del `Manager` (`NewManagerWithRunner`, `manager.go:42`), lo que permite
testearlos sin Docker. Cobertura del paquete: 100%.

> **Adopcion parcial.** Hoy solo lo usan `compose` y `services`. Los paquetes `doctor`,
> `preflight`, `proxy`, `dns`, `observe` y `compose/logs` mantienen su propio helper de
> `exec`, con timeouts distintos (3 s, 5 s, 10 s o ninguno). Unificarlos es una de las
> tareas pendientes de mayor retorno.

## 6. Base de datos (SQLite)

DevHerd usa **dos bases SQLite independientes**: la principal (proyectos y dominios) y una
separada para observabilidad (`internal/observe/store.go:101`).

- `internal/database/db.go:21` — `Manager.Ensure` crea/abre la DB y ejecuta las
  migraciones pendientes. Detecta si el archivo no existia para reportar "created" vs
  "migrated".
- `internal/database/db.go:49` — `migrate` mantiene una tabla `schema_migrations` y aplica
  solo las versiones que faltan, cargadas desde `//go:embed migrations/*.sql`
  (`migrations.go:12`). Tolera bases legacy registrando la baseline.
  **Contrato importante**: no hay transaccion por migracion ni rollback, asi que el SQL de
  cada migracion debe ser idempotente (`CREATE ... IF NOT EXISTS`).
- `internal/database/db.go:107` — `Manager.Open` abre con `modernc.org/sqlite`, activando
  `foreign_keys` y `busy_timeout`.
- `internal/database/migrations/0001_init.sql` — baseline con las tablas `settings`,
  `parks`, `projects`, `project_domains`, `runtime_preferences`, `services`,
  `sentry_configs` y `events`. Usa `journal_mode = WAL`.
  De estas, **solo `parks`, `projects` y `project_domains` tienen codigo Go que las use**;
  el resto estan declaradas para iteraciones futuras.

La base de Observe, en cambio, **no tiene versionado**: `internal/observe/migrations.go`
solo embebe `schema.sql` y `Ensure` lo reejecuta completo en cada invocacion.
- `internal/database/projects.go` — operaciones de dominio:
  - `ProjectRecord` (`projects.go:14`): modelo de proyecto.
  - `InsertPark` (`projects.go:25`), `UpsertProject` (`projects.go:38`),
    `ListProjects` (`projects.go:121`), `FindProjectByPath` (`projects.go:169`),
    `SetPrimaryDomain` (`projects.go:207`), `PruneDetectedProjectsUnderPath`
    (`projects.go:97`).
  - La asignacion de dominios es transaccional y valida unicidad con
    `ensureDomainAvailable` (`projects.go:273`).

## 7. Deteccion de proyectos

`internal/detector/detector.go`:

- `Discover(root)` (`detector.go:31`) escanea el directorio raiz y sus hijos directos
  (ignorando `node_modules` y carpetas ocultas) y devuelve los proyectos detectados,
  filtrando anidados con `filterNestedProjects` (`detector.go:128`).
- `DetectProject(path)` (`detector.go:80`) inspecciona un directorio y construye un
  `featureSet` segun la presencia de archivos:
  - Laravel: `artisan` + `composer.json`.
  - Node: `package.json` (Vue si tiene dependencia `vue`).
  - Python/Flask: `requirements.txt`, `pyproject.toml`, `app.py`.
  - Go: `go.mod`.
  - Docker: archivos compose o `Dockerfile`.
- `describeFramework`/`describeStack`/`describeRuntime` (`detector.go:285,259,308`) mapean
  el `featureSet` a strings como `vue+flask`, `laravel`, `php+node+python+docker`, etc.

El framework `vue+flask` es especial: dispara rutas de proxy predefinidas (ver seccion 9).

## 8. Resolucion y ejecucion de Compose

`internal/compose/project.go`:

- `Project` (`project.go:34`) modela un proyecto: `Root`, `ComposeFiles`, `EnvFile`,
  `Source`, `ProjectName`, `LegacyProjectName`, `Proxy`.
- `ResolveProject(input)` (`project.go:67`):
  1. Si existe `.devherd.yml` lo parsea como manifiesto (`resolveManifestProject`,
     `project.go:198`), que define archivos compose, env file y metadata de proxy
     (`domain`, `service`, `port`).
  2. Si no, autodetecta el primer archivo compose soportado
     (`supportedComposeFiles`, `project.go:20`).
- **Nombre de proyecto estable por ruta**: `ProjectNameForPath` (`project.go:335`) genera
  `devherd-<slug>-<sha1[:8]>` a partir de la ruta absoluta. Esto aisla clones con el mismo
  nombre de carpeta. `LegacyProjectNameForPath` (`project.go:354`) preserva el nombre
  antiguo para `down`/`stop` de stacks creados antes.
- Ejecucion: `UpProject` (`docker compose up --build -d`), `DownProject`, `StopProject`
  (`project.go:138-196`). `composeArgs` (`project.go:276`) construye los flags
  `--project-name`, `--env-file` y `-f` por cada archivo compose.
- `Plan` (`project.go:294`) devuelve el proyecto resuelto y el comando docker base sin
  ejecutar nada.

### 8.1 Streaming de logs

`internal/compose/logs.go` implementa `devherd logs`:

- `LogsOptions` (`logs.go:10`) agrupa `Follow`, `Tail` y `Services`.
- `LogsArgs(project, opts)` (`logs.go:18`) es una funcion **pura** que construye los
  argumentos de `docker compose ... logs ...` (anadiendo `--follow`/`--tail` segun
  corresponda); se mantiene aislada para testearla sin Docker.
- `LogsProject` (`logs.go:36`) y `Logs` (`logs.go:48`) ejecutan el comando. A diferencia de
  `run`/`UpProject`, **no almacenan la salida en buffer**: conectan `cmd.Stdout`/`cmd.Stderr`
  directamente a los writers indicados, lo que permite el streaming en vivo de `--follow`.

El comando CLI (`internal/cli/logs.go`) resuelve el proyecto y, si DevHerd esta
inicializado, alinea los archivos compose con los del proxy externo y observe
(`appendObserveOverride`) para cubrir todos los servicios en ejecucion, antes de llamar a
`LogsProject`.

### 8.2 Generacion de compose (`scaffold`)

`internal/scaffold/scaffold.go` genera un stack Compose para repositorios que aun no estan
contenerizados. Escribe dos archivos con nombre propio para no pisar los del usuario:
`docker-compose.devherd.yml` y `.devherd.yml` (`scaffold.go:20-23`).

- `Detect(path)` (`scaffold.go:54-90`) reconoce, en orden: el combo `vue+flask` (buscando
  subdirectorios), y luego en la raiz `laravel`, `vue`, `flask`, `node` y `go`.
- El plan de Laravel (`laravelPlan`, `scaffold.go:170-207`) marca `Plan.Complete`: deriva
  la base de datos y las credenciales del `.env` del propio repositorio, anade Redis
  siempre y un servicio Vite si hay `package.json`.
- `AssignHostPorts` (`scaffold.go:424-437`) busca puertos de host libres a partir del
  preferido de cada stack, para no colisionar con proyectos ya levantados.
- `RenderCompose` (`scaffold.go:463-512`) emite los comandos en forma exec
  (`["sh","-c",...]`) y escapa `$` como `$$` (`composeEscape`, `scaffold.go:560`) para que
  docker compose no interpole las variables de los scripts.
- `RenderManifest` (`scaffold.go:516-531`) declara `proxy.service` y `proxy.port` cuando
  hay un unico servicio de aplicacion, de modo que el proxy externo pueda enrutar sin
  depender del `switch` por framework.

Desde la CLI, `ensureComposeOrScaffold` (`internal/cli/scaffold.go:110-132`) permite que
`up` ofrezca generar el compose cuando el proyecto no tiene ninguno. Nota de mantenimiento:
esa deteccion compara el **texto** del error de `ResolveProject`, asi que reformular ese
mensaje rompe la integracion sin fallar la compilacion.

## 9. Flujo del proxy reverso

DevHerd soporta dos drivers de proxy, controlados por `cfg.Proxy.Driver`:

### 9.1 Caddy en host (`caddy`)

`internal/proxy/caddy.go`:

- `Renderer` (`caddy.go:33`) renderiza un `Caddyfile` a partir de la plantilla embebida
  `Caddyfile.tmpl` (`caddy.go:20`, `internal/proxy/Caddyfile.tmpl`).
- `Renderer.projectSite` (`caddy.go:119`) mapea el framework del proyecto a rutas
  reverse_proxy hacia `localhost`:
  - `vue+flask`: `/api/*` -> `127.0.0.1:8000`, `/*` -> `127.0.0.1:5173`.
  - `flask`: `/*` -> `127.0.0.1:8000`.
  - `vue`: `/*` -> `127.0.0.1:5173`.
- `Renderer.Write` (`caddy.go:85`) escribe el Caddyfile en `~/.local/share/devherd/proxy/Caddyfile`.
- `Renderer.Apply` (`caddy.go:94`) valida (`sudo caddy validate`) y recarga/arranca Caddy
  (`sudo caddy reload`/`start`). Requiere `sudo` y el binario `caddy` en PATH.
- La resolucion de nombres se hace via `/etc/hosts` (ver seccion 10).

### 9.2 Caddy en Docker externo (`caddy-docker-external`)

`internal/proxy/external.go` y `bootstrap.go`. Aqui DevHerd administra un stack propio
(directorio `local_proxy` bajo `DataDir`) cuyo contenedor Caddy se llama `infra_caddy`
(`config.Proxy.ExternalContainerName`), conectado a una red Docker compartida.

Constantes clave (`external.go:20-28`): `DriverCaddyDockerExternal`,
`ExternalProxyCaddyfile`, `ExternalProxyComposeFile`, `ManagedComposeOverrideFile`
(`.devherd.proxy.override.yml`).

Flujo de `proxy apply` (driver externo), orquestado en `internal/cli/proxy.go:64-95`:

1. `BuildExternalProject(cfg, project)` (`external.go:99`) calcula el dominio efectivo
   (`effectiveDomain`, `external.go:277`), el prefijo de alias y las rutas. Si el
   manifiesto declara `proxy.service`+`proxy.port`, usa esa ruta; si no, cae en la regla
   especial para `vue+flask`.
2. `EnsureComposeOverride` (`external.go:141`) escribe `.devherd.proxy.override.yml` en la
   raiz del proyecto. Ese override conecta los servicios del proyecto a la red externa
   (`cfg.Proxy.ExternalNetwork`, por defecto `infra_web`) con aliases por servicio.
3. `ConnectProject` (`external.go:178`) crea la red externa si falta
   (`ensureExternalProxyNetwork`, `external.go:493`) y conecta cada contenedor del proyecto
   a esa red con su alias (`docker network connect --alias ...`).
4. `ApplyExternalProxy` (`external.go:206`):
   - Hace bootstrap de los assets del proxy externo (`BootstrapExternalProxy`).
   - Fusiona los bloques de sitios administrados en el `Caddyfile` del local_proxy
     (`mergeExternalProxyConfig`/`renderExternalSite`, `external.go:328-383`).
   - Levanta el contenedor (`docker compose up -d`) y ejecuta dentro del contenedor
     `caddy validate` + `caddy reload`.
5. `syncManagedDomains` (`proxy.go:127`) sincroniza `/etc/hosts`.

Al bajar un proyecto (`down`), `RemoveExternalProxy` (`external.go:239`) elimina los
bloques de dominio del Caddyfile y recarga Caddy; ademas se borra el override y se
desconecta de la red (ver `internal/cli/down.go:45-69`).

### 9.3 Bootstrap de assets del proxy externo

`internal/proxy/bootstrap.go`:

- `BootstrapExternalProxy` / `BootstrapExternalProxyWithOptions` (`bootstrap.go:26,30`)
  renderizan las plantillas embebidas (`templates/proxy-external/`,
  `templates/proxy-external/embed.go`) y escriben en
  `cfg.Proxy.ExternalDir` (por defecto `~/.local/share/devherd/local_proxy`):
  `docker-compose.yml`, `Caddyfile`, `.env` y `.env.example`.
- `ensureManagedFile` (`bootstrap.go:91`) es idempotente: reusa si el contenido coincide
  y solo reescribe con `--force`.

## 10. DNS local (`/etc/hosts`)

`internal/dns/hosts.go`:

- `SyncHosts(domains)` (`hosts.go:46`) reescribe un bloque administrado delimitado por
  `# devherd start` / `# devherd end` (`hosts.go:11-14`) apuntando todos los dominios a
  `127.0.0.1`. Usa un archivo temporal y `sudo cp` para reemplazar `/etc/hosts`.
- `mergeManagedBlock` (`hosts.go:86`) preserva el resto del archivo y reemplaza solo el
  bloque administrado.

## 11. Servicios compartidos

`internal/services/manager.go`:

- `Manager` (`manager.go:31`) administra un stack compose en
  `~/.local/share/devherd/compose/shared-services/docker-compose.yml`.
- Servicios soportados: `redis` y `mailpit` (`supportedServices`, `manager.go:29`).
- `Start`/`Stop`/`Status` (`manager.go:55,71,83`) corren `docker compose ... up -d|stop|ps`
  con `--project-name devherd_shared`.
- Crean/garantizan la red `infra_net` (`NetworkName`, `manager.go:24`; `ensureNetwork`,
  `manager.go:129`).
- El contenido del compose se embebe desde un `.yml` real via `//go:embed`
  (`composeContent` <- `shared-services.compose.yml`, `manager.go:21`): Redis 7-alpine
  (`infra_redis`, puerto `127.0.0.1:6379`) y Mailpit (`infra_mailpit`, puertos
  `127.0.0.1:1025`/`8025`).

## 12. Preflight / inspect

`internal/preflight/preflight.go`:

- `Inspect(ctx, targetPath, cfg)` (`preflight.go:94`) resuelve el proyecto y produce un
  `Report` con `Finding`s de severidad `ok`/`warn`/`fail` (`Severity`, `preflight.go:23`).
- Comprobaciones: nombres de contenedor que colisionan con otro proyecto compose
  (`inspectContainerNames`), puertos publicados ya en uso (`inspectPorts`), volumenes
  externos (`inspectVolumes`), reglas de entorno de Laravel/Redis (`inspectLaravelEnv`),
  redes compartidas (`inspectSharedNetworks`) y estado del proxy externo
  (`inspectExternalProxy`).
- `Report.HasFailures`/`HasWarnings` (`preflight.go:42,46`) los usa `up` para decidir si
  abortar.

`devherd up` ejecuta este preflight automaticamente (`internal/cli/up.go:45-49`,
`runUpPreflight`, `up.go:71`), salvo `--no-inspect`; con `--force` continua pese a
fallos.

## 13. Doctor

`internal/doctor/doctor.go`:

- `RunWithConfig(ctx, cfg)` (`doctor.go:53`) ejecuta checks segun el driver de proxy:
  - Comunes: rutas locales, `docker` CLI, daemon Docker, modo de engine (requiere Linux),
    `docker compose` (`doctor.go:59-65`).
  - Driver `caddy-docker-external`: directorio/compose/Caddyfile del local_proxy, redes
    `infra_web` e `infra_net`, sufijo administrado, puerto 80 del contenedor proxy
    (`doctor.go:67-76`).
  - Driver `caddy`: binario `caddy`, `dnsmasq` opcional, puertos 80 y 443
    (`doctor.go:78-83`).
- Cada check produce `Check{Name, Status, Message}` (`doctor.go:27`). `up`/`inspect`/
  `doctor` comparten el formato de salida.

## 14. Observe (observabilidad local)

`internal/cli/observe.go` + `internal/observe/`:

- Subcomandos registrados en `newObserveCmd` (`observe.go:25-47`): `start`, `status`,
  `open`, `dsn`, `attach`, `detach`, `scan`, `containers`, `timeline`, `cleanup`,
  `alert`, `issues`, `events`.
- `observe start` (`observe.go:50`) arranca un servidor HTTP (`observe.NewServer`) que
  recibe eventos tipo Sentry en un DSN local y los agrupa en *issues*. Usa una base SQLite
  **separada** (`observe.DefaultDBPath`, `observe.NewManager`, `observe.go:818-820`).
- `observe attach` (`observe.go:186`) genera un override compose local
  (`observe.EnsureComposeOverride`) que inyecta el DSN local y la configuracion de Sentry
  en los servicios elegidos del proyecto, segun el `--stack`.
- `appendObserveOverride` (`internal/cli/compose_runtime.go:67`) hace que `up`/`stop`/
  `down` incluyan automaticamente el override de observe si existe en la raiz del proyecto.
- Alertas locales: `observe alert add/list/remove/deliveries` (`observe.go:431-597`) con
  tipos `new-issue`, `error-rate`, `container-exit`, `container-restart`.

## 15. Sentry (placeholder)

`internal/cli/sentry.go` son 92 lineas que **no importan ningun paquete interno de
DevHerd**: es un cascaron completo.

- `sentry init <project> --stack <stack> --dry-run` (`sentry.go:24`) imprime un plan de
  pasos **estatico y hardcodeado**: no inspecciona el proyecto ni lee la configuracion, y
  el valor de `--stack` no se valida. El modo "apply" retorna `notImplemented`
  (`sentry.go:52`).
- `sentry set-dsn` y `sentry test` son stubs que devuelven `notImplemented` y estan
  marcados `Hidden: true` (`sentry.go:69`, `:86`), de modo que no aparecen en la ayuda.

El paquete `internal/sentry/` es un `doc.go` vacio, `templates/sentry/` un README no
embebido, y la tabla `sentry_configs` no la lee ni escribe nadie.

**La funcionalidad tipo-Sentry real vive integramente en `internal/observe`**, y ambos
subsistemas estan desconectados entre si: `observe attach` genera su DSN local sin pasar
por `sentry set-dsn` ni por `sentry_configs`.

## 16. Tipos y funciones clave (referencia rapida)

| Tipo / funcion | Archivo:linea | Rol |
|----------------|---------------|-----|
| `cli.Execute` | `internal/cli/root.go:10` | Arranque de la CLI |
| `appContext` / `loadAppContext` | `internal/cli/app_context.go:15,21` | Config + DB compartidos |
| `config.Config` / `config.Default` | `internal/config/config.go:9,38` | Modelo y defaults de config |
| `config.Paths` / `ResolvePaths` | `internal/config/paths.go:9,20` | Rutas XDG |
| `database.ProjectRecord` | `internal/database/projects.go:14` | Proyecto persistido |
| `detector.DetectProject` | `internal/detector/detector.go:80` | Deteccion de stack |
| `compose.Project` / `ResolveProject` | `internal/compose/project.go:34,67` | Modelo y resolucion compose |
| `compose.ProjectNameForPath` | `internal/compose/project.go:335` | Aislamiento por ruta |
| `runner.Runner` / `runner.Cmd` | `internal/runner/runner.go:19,24` | Seam de ejecucion de comandos |
| `scaffold.Detect` / `scaffold.Write` | `internal/scaffold/scaffold.go:54,584` | Generacion de compose |
| `runSiblingCommand` | `internal/cli/serve.go:57` | Composicion de comandos (`serve`, `scaffold --up`) |
| `proxy.Renderer` | `internal/proxy/caddy.go:33` | Render Caddy en host |
| `proxy.ApplyExternalProxy` | `internal/proxy/external.go:206` | Aplicar proxy Docker externo |
| `proxy.BootstrapExternalProxy` | `internal/proxy/bootstrap.go:26` | Crear assets del local_proxy |
| `dns.SyncHosts` | `internal/dns/hosts.go:46` | Bloque administrado en /etc/hosts |
| `services.Manager` | `internal/services/manager.go:31` | Redis/Mailpit compartidos |
| `preflight.Inspect` | `internal/preflight/preflight.go:94` | Chequeos de colision |
| `doctor.RunWithConfig` | `internal/doctor/doctor.go:53` | Prerequisitos del host |
| `observe.NewServer` | `internal/cli/observe.go:71` | Collector HTTP local |

## 17. Notas de diseno

- **Sin estado en memoria entre comandos**: cada comando reconstruye su contexto desde
  disco. Esto simplifica el modelo, a costa de reabrir SQLite por invocacion.
- **Plantillas embebidas** (`go:embed`): el binario es autocontenido; no depende de
  archivos de plantilla en disco para Caddy o el proxy externo.
- **SQLite sin CGO** (`modernc.org/sqlite`): habilita `CGO_ENABLED=0`, cross-compilacion
  trivial, imagen distroless y un CI sin matriz de toolchains nativos.
- **Idempotencia**: bootstrap de proxy y servicios reescriben/reusan archivos de forma
  determinista; las migraciones SQLite usan `CREATE TABLE IF NOT EXISTS`.
- **Aislamiento de clones**: el nombre de proyecto compose se deriva de la ruta absoluta
  (hash SHA1), evitando colisiones entre clones con el mismo nombre de carpeta.
- **Bloques administrados con marcadores explicitos**: tanto `/etc/hosts` como el
  Caddyfile del proxy externo delimitan lo que gestiona DevHerd, preservando el resto del
  archivo. El Caddyfile conserva ademas una pasada de migracion desde el formato antiguo
  basado en conteo de llaves.
- **Separacion stdout/stderr**: la salida de producto va a stdout y el diagnostico a
  stderr, de modo que cada flujo se puede redirigir por separado.
- **Composicion de comandos sin duplicar logica**: `serve` y `scaffold --up` invocan a sus
  comandos hermanos a traves de `runSiblingCommand` en vez de reimplementar sus pasos.

### Puntos de friccion conocidos

Estos comportamientos son reales y conviene tenerlos presentes al tocar el codigo; el
detalle y su priorizacion estan en [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md#10-riesgos-y-deuda-priorizados):

- `loadAppContext` **crea directorios y la base de datos** aunque el comando sea de solo
  lectura.
- `prepareComposeProject` (`compose_runtime.go:26-28`) traga el error de
  `resolveExternalProject` y devuelve el proyecto **sin** los overrides, divergiendo en
  silencio de lo que `proxy apply` espera.
- `plan` e `inspect` no incluyen los overrides, asi que el comando que muestran puede
  diferir del que ejecuta `up`.
- El driver `caddy` en host ignora la metadata `proxy` del manifiesto y regenera el
  Caddyfile completo en cada aplicacion.
- El Caddyfile del proxy externo se escribe **antes** de validarse, sin rollback si la
  validacion falla.
