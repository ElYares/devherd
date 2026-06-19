# Arquitectura de DevHerd

Esta guia describe la arquitectura interna de DevHerd: el layout de paquetes, las
responsabilidades de cada uno, el flujo de datos a traves del proxy y los tipos y
funciones clave. Todas las referencias apuntan a archivos y lineas reales del
repositorio.

> Toda la documentacion se basa en el codigo actual del repositorio. DevHerd es un
> producto en estado MVP/alpha (`version.Version = "0.1.0-alpha"`, ver
> `internal/version/version.go:4`).

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
        |  paths) | | schema) | | stacks)  | | compose| |  checks)|
        +---------+ +---------+ +----------+ +-------+ +---------+
              |                                  |        |
              v                                  v        v
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
        | (prereqs) |   |(collector|  | (scaffold|
        |           |   |  + panel)|  |   stub)  |
        +-----------+   +---------+   +----------+
```

Flujo tipico de un usuario:

```
init -> park -> (plan / inspect) -> up -> proxy apply -> open
                                     |
                                     +-> service start redis|mailpit
                                     +-> observe attach + observe start
```

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
| `proxy` | `internal/proxy/` | Render y aplicacion del proxy reverso: Caddy en host y `caddy-docker-external`. |
| `dns` | `internal/dns/` | Sincronizacion del bloque administrado en `/etc/hosts`. |
| `services` | `internal/services/` | Stack compose de servicios compartidos (Redis, Mailpit) en red `infra_net`. |
| `doctor` | `internal/doctor/` | Validacion de prerequisitos del host (Docker, Caddy, puertos, redes). |
| `observe` | `internal/observe/` | Collector local de errores, panel web, store SQLite separada, alertas, correlacion Docker. |
| `sentry` | `internal/sentry/` | Integracion con Sentry (mayormente scaffold/stub en el MVP). |
| `version` | `internal/version/` | Metadatos de version. `String()` devuelve la version semantica; `Long()` la enriquece con commit y fecha (`Version (commit X, built Y)`), usada por `devherd --version`. Las variables `Version/Commit/Date` se inyectan en build via `-ldflags` (ver `Makefile`). |
| `templates/` | `templates/` | Plantillas embebidas (`go:embed`) del proxy externo, Caddy, nginx y Sentry. |

Algunos paquetes (`internal/api`, `internal/logs`, `internal/runtimes`, `internal/sentry`)
contienen principalmente `doc.go` como placeholders de iteraciones futuras.

## 4. Punto de entrada y registro de comandos

- `cmd/devherd/main.go:10` — `main()` invoca `cli.Execute()`.
- `internal/cli/root.go:10` — `Execute()` ejecuta `newRootCmd()`.
- `internal/cli/root.go:14` — `newRootCmd()` crea el comando raiz `devherd` y registra
  todos los subcomandos via `cmd.AddCommand(...)` (`root.go:25-42`):
  `init`, `doctor`, `park`, `list`, `domain`, `proxy`, `plan`, `inspect`, `up`, `stop`,
  `down`, `open`, `logs`, `service`, `observe`, `sentry`.

El root configura `SilenceErrors` y `SilenceUsage` para imprimir errores limpios desde
`main` (`root.go:20-21`).

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

- `internal/cli/app_context.go:14` — el tipo `appContext` agrupa `config.Paths`,
  `config.Config` y `*sql.DB`.
- `internal/cli/app_context.go:20` — `loadAppContext(ctx)`:
  1. Resuelve rutas XDG (`config.ResolvePaths`).
  2. Crea los directorios locales (`paths.Ensure`).
  3. Carga `config.json`; si no existe, devuelve el error
     *"DevHerd is not initialized. Run `devherd init` first"* (`app_context.go:33`).
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
`config.go:119-135`).

## 6. Base de datos (SQLite)

- `internal/database/db.go:21` — `Manager.Ensure` crea/abre la DB y aplica el esquema
  embebido (`schema.sql`). Detecta si el archivo no existia para reportar "created" vs
  "migrated".
- `internal/database/db.go:47` — `Manager.Open` abre con `modernc.org/sqlite`, activando
  `foreign_keys` y `busy_timeout`.
- `internal/database/schema.sql` — define las tablas: `settings`, `parks`, `projects`,
  `project_domains`, `runtime_preferences`, `services`, `sentry_configs`, `events`. Usa
  `journal_mode = WAL`.
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

- `Project` (`project.go:30`) modela un proyecto: `Root`, `ComposeFiles`, `EnvFile`,
  `Source`, `ProjectName`, `LegacyProjectName`, `Proxy`.
- `ResolveProject(input)` (`project.go:63`):
  1. Si existe `.devherd.yml` lo parsea como manifiesto (`resolveManifestProject`,
     `project.go:194`), que define archivos compose, env file y metadata de proxy
     (`domain`, `service`, `port`).
  2. Si no, autodetecta el primer archivo compose soportado
     (`supportedComposeFiles`, `project.go:16`).
- **Nombre de proyecto estable por ruta**: `ProjectNameForPath` (`project.go:344`) genera
  `devherd-<slug>-<sha1[:8]>` a partir de la ruta absoluta. Esto aisla clones con el mismo
  nombre de carpeta. `LegacyProjectNameForPath` (`project.go:363`) preserva el nombre
  antiguo para `down`/`stop` de stacks creados antes.
- Ejecucion: `UpProject` (`docker compose up --build -d`), `DownProject`, `StopProject`
  (`project.go:134-192`). `composeArgs` (`project.go:272`) construye los flags
  `--project-name`, `--env-file` y `-f` por cada archivo compose.
- `Plan` (`project.go:290`) devuelve el proyecto resuelto y el comando docker base sin
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

`internal/proxy/external.go` y `bootstrap.go`. Aqui DevHerd administra un stack
"local_proxy" (un contenedor Caddy en una red Docker compartida).

Constantes clave (`external.go:19-28`): `DriverCaddyDockerExternal`,
`ExternalProxyCaddyfile`, `ExternalProxyComposeFile`, `ManagedComposeOverrideFile`
(`.devherd.proxy.override.yml`).

Flujo de `proxy apply` (driver externo), orquestado en `internal/cli/proxy.go:59-90`:

1. `BuildExternalProject(cfg, project)` (`external.go:90`) calcula el dominio efectivo
   (`effectiveDomain`, `external.go:266`), el prefijo de alias y las rutas. Si el
   manifiesto declara `proxy.service`+`proxy.port`, usa esa ruta; si no, cae en la regla
   especial para `vue+flask`.
2. `EnsureComposeOverride` (`external.go:132`) escribe `.devherd.proxy.override.yml` en la
   raiz del proyecto. Ese override conecta los servicios del proyecto a la red externa
   (`cfg.Proxy.ExternalNetwork`, por defecto `infra_web`) con aliases por servicio.
3. `ConnectProject` (`external.go:169`) crea la red externa si falta
   (`ensureExternalProxyNetwork`, `external.go:441`) y conecta cada contenedor del proyecto
   a esa red con su alias (`docker network connect --alias ...`).
4. `ApplyExternalProxy` (`external.go:195`):
   - Hace bootstrap de los assets del proxy externo (`BootstrapExternalProxy`).
   - Fusiona los bloques de sitios administrados en el `Caddyfile` del local_proxy
     (`mergeExternalProxyConfig`/`renderExternalSite`, `external.go:317-369`).
   - Levanta el contenedor (`docker compose up -d`) y ejecuta dentro del contenedor
     `caddy validate` + `caddy reload`.
5. `syncManagedDomains` (`proxy.go:122`) sincroniza `/etc/hosts`.

Al bajar un proyecto (`down`), `RemoveExternalProxy` (`external.go:228`) elimina los
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

- `SyncHosts(domains)` (`hosts.go:17`) reescribe un bloque administrado delimitado por
  `# devherd start` / `# devherd end` (`hosts.go:11-14`) apuntando todos los dominios a
  `127.0.0.1`. Usa un archivo temporal y `sudo cp` para reemplazar `/etc/hosts`.
- `mergeManagedBlock` (`hosts.go:50`) preserva el resto del archivo y reemplaza solo el
  bloque administrado.

## 11. Servicios compartidos

`internal/services/manager.go`:

- `Manager` (`manager.go:23`) administra un stack compose en
  `~/.local/share/devherd/compose/shared-services/docker-compose.yml`.
- Servicios soportados: `redis` y `mailpit` (`supportedServices`, `manager.go:21`).
- `Start`/`Stop`/`Status` (`manager.go:40,56,68`) corren `docker compose ... up -d|stop|ps`
  con `--project-name devherd_shared`.
- Crean/garantizan la red `infra_net` (`NetworkName`, `manager.go:16`; `ensureNetwork`,
  `manager.go:127`).
- El contenido del compose esta embebido como string (`composeContent`, `manager.go:169`):
  Redis 7 (`infra_redis`, puerto `127.0.0.1:6379`) y Mailpit (`infra_mailpit`, puertos
  `1025`/`8025`).

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

`devherd up` ejecuta este preflight automaticamente (`internal/cli/up.go:36-53`,
`runUpPreflight`, `up.go:62`), salvo `--no-inspect`; con `--force` continua pese a
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

## 15. Sentry (estado MVP)

`internal/cli/sentry.go`:

- `sentry init <project> --stack <stack> --dry-run` (`sentry.go:24`) imprime un plan de
  pasos sin tocar archivos. El modo "apply" todavia retorna `notImplemented`
  (`sentry.go:52`).
- `sentry set-dsn` y `sentry test` son stubs que devuelven `notImplemented` y estan
  marcados `Hidden: true` (`internal/cli/sentry.go`), de modo que no aparecen en la ayuda
  hasta implementarse.

## 16. Tipos y funciones clave (referencia rapida)

| Tipo / funcion | Archivo:linea | Rol |
|----------------|---------------|-----|
| `cli.Execute` | `internal/cli/root.go:10` | Arranque de la CLI |
| `appContext` / `loadAppContext` | `internal/cli/app_context.go:14,20` | Config + DB compartidos |
| `config.Config` / `config.Default` | `internal/config/config.go:9,38` | Modelo y defaults de config |
| `config.Paths` / `ResolvePaths` | `internal/config/paths.go:9,20` | Rutas XDG |
| `database.ProjectRecord` | `internal/database/projects.go:14` | Proyecto persistido |
| `detector.DetectProject` | `internal/detector/detector.go:80` | Deteccion de stack |
| `compose.Project` / `ResolveProject` | `internal/compose/project.go:30,63` | Modelo y resolucion compose |
| `compose.ProjectNameForPath` | `internal/compose/project.go:344` | Aislamiento por ruta |
| `proxy.Renderer` | `internal/proxy/caddy.go:33` | Render Caddy en host |
| `proxy.ApplyExternalProxy` | `internal/proxy/external.go:195` | Aplicar proxy Docker externo |
| `proxy.BootstrapExternalProxy` | `internal/proxy/bootstrap.go:26` | Crear assets del local_proxy |
| `dns.SyncHosts` | `internal/dns/hosts.go:17` | Bloque administrado en /etc/hosts |
| `services.Manager` | `internal/services/manager.go:23` | Redis/Mailpit compartidos |
| `preflight.Inspect` | `internal/preflight/preflight.go:94` | Chequeos de colision |
| `doctor.RunWithConfig` | `internal/doctor/doctor.go:53` | Prerequisitos del host |
| `observe.NewServer` | `internal/cli/observe.go:71` | Collector HTTP local |

## 17. Notas de diseno

- **Sin estado en memoria entre comandos**: cada comando reconstruye su contexto desde
  disco. Esto simplifica el modelo, a costa de reabrir SQLite por invocacion.
- **Plantillas embebidas** (`go:embed`): el binario es autocontenido; no depende de
  archivos de plantilla en disco para Caddy o el proxy externo.
- **Idempotencia**: bootstrap de proxy y servicios reescriben/reusan archivos de forma
  determinista; las migraciones SQLite usan `CREATE TABLE IF NOT EXISTS`.
- **Aislamiento de clones**: el nombre de proyecto compose se deriva de la ruta absoluta
  (hash SHA1), evitando colisiones entre clones con el mismo nombre de carpeta.
</content>
</invoke>
