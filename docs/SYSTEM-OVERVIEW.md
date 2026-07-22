# DevHerd: análisis completo del sistema

> Revisión integral del repositorio en el commit `76a8a24` (2026-07-21).
> Todo lo que sigue está verificado contra el código; las métricas de tests y build
> son salidas reales de comandos, no estimaciones. Las afirmaciones llevan
> referencia `archivo:línea`.

Este documento responde a tres preguntas: **qué es DevHerd**, **cómo está construido
por dentro** y **en qué estado real se encuentra**. Para aprender a usar la
herramienta, ver [USAGE.md](USAGE.md); para el detalle de paquetes y tipos, ver
[ARCHITECTURE.md](ARCHITECTURE.md).

---

## 1. Qué es DevHerd

Una **CLI en Go** que hace de plataforma de desarrollo local "Ubuntu-first",
inspirada en Herd. Coordina cuatro cosas que normalmente se montan a mano:

1. **Proyectos Docker Compose** — levantar, parar, bajar y seguir logs, con nombres
   de proyecto aislados por ruta.
2. **Dominios locales** — `proyecto.test` o `proyecto.localhost` publicados por un
   proxy Caddy, en el host o en un contenedor administrado.
3. **Servicios compartidos** — Redis y Mailpit en una red Docker común.
4. **Observabilidad local** — un collector propio compatible-ish con Sentry, con
   panel web, agrupación de errores en issues y correlación con contenedores Docker.

A eso se suma un **generador de `docker-compose`** (`scaffold`) para repos que aún
no están contenerizados.

**No es un daemon.** Cada invocación resuelve rutas, abre SQLite, ejecuta y sale.
La única excepción es `devherd observe start`, un servidor HTTP en foreground
(`internal/observe/server.go:53`).

### Identidad y dependencias

| Dato | Valor |
|---|---|
| Módulo | `github.com/devherd/devherd` |
| Versión | `0.1.0-alpha` (`internal/version/version.go:7`) |
| Go declarado | `1.25.0` (toolchain local en la máquina de desarrollo: 1.26.3) |
| Dependencias directas | **3** |
| Código Go | ~12.900 líneas |

Las tres dependencias directas:

| Dependencia | Uso |
|---|---|
| `github.com/spf13/cobra` v1.10.2 | Framework de comandos (`internal/cli/root.go:34-53`) |
| `gopkg.in/yaml.v3` v3.0.1 | Manifiesto `.devherd.yml` y parseo de archivos compose |
| `modernc.org/sqlite` v1.50.0 | SQLite **en Go puro, sin CGO** (`internal/database/db.go:10`) |

La elección de `modernc.org/sqlite` es la decisión de infraestructura con mayor
retorno del proyecto: permite `CGO_ENABLED=0`, cross-compilación trivial, imagen
distroless y CI sin matriz de toolchains nativos.

---

## 2. Mapa de subsistemas

```
                        cmd/devherd/main.go
                     (15 líneas: Execute + exit 1)
                                 │
                                 ▼
                    internal/cli  ── 18 comandos raíz
        root.go: flags globales --verbose / --log-json
        app_context.go: paths XDG + config.json + SQLite
                                 │
     ┌───────────┬───────────┬───┴────────┬────────────┬──────────┐
     ▼           ▼           ▼            ▼            ▼          ▼
  config     database     detector     compose     preflight   doctor
  (XDG +     (SQLite      (stacks y    (resolve,   (colisiones (prereqs
   JSON)      migrado)     frameworks)  naming,     de puertos, del host)
                                        args)       nombres…)
     │                                     │
     │            ┌────────────────────────┴──────────┐
     ▼            ▼                                   ▼
  scaffold     proxy ── caddy (host, sudo)         observe
  (genera      │     └─ caddy-docker-external      (collector HTTP,
   compose)    │        (contenedor infra_caddy)    SQLite propia,
               ▼                                    panel, alertas,
              dns  (bloque administrado             correlación Docker)
                    en /etc/hosts, sudo)
                                 │
                             services
                    (redis + mailpit en infra_net)
```

Flujo típico de un usuario:

```
init → park → [scaffold] → plan/inspect → up → proxy apply → open
                                           │
                                           ├→ service start redis|mailpit
                                           └→ observe attach + observe start
```

O en un solo paso: `devherd serve` encadena `up` + `proxy apply` + `open`
(`internal/cli/serve.go:22-51`).

### Estado por subsistema

| Subsistema | Estado | Nota |
|---|---|---|
| Ciclo de vida Compose | **Sólido** | Naming estable por ruta, manifiesto, overrides |
| Proxy `caddy-docker-external` | **Sólido** | Es el camino recomendado y el más ejercitado |
| Proxy `caddy` en host | **Limitado** | Sólo 3 frameworks, ignora el manifiesto (§5.2) |
| Proxy `nginx` | **No existe** | El valor se acepta y persiste, pero no hay driver |
| DNS `/etc/hosts` | **Sólido** | Validado, idempotente, sólo IPv4 |
| Detector | **Estrecho** | 1 nivel de profundidad, catálogo corto (§6) |
| Scaffold | **Funcional, joven** | Laravel completo; varios bordes ásperos (§7) |
| Servicios compartidos | **Mínimo** | Sólo Redis y Mailpit |
| Observe | **Funcional con límites** | Ingesta propia OK; SDK Sentry real dudoso (§8) |
| Sentry (`devherd sentry`) | **Placeholder** | No hace nada funcional (§8.5) |

---

## 3. El núcleo transversal

### 3.1 Contexto de aplicación

Casi todos los comandos empiezan por `loadAppContext` (`internal/cli/app_context.go:21-63`),
que en orden: resuelve rutas XDG → **crea 6 directorios** → carga `config.json` →
aplica defaults de rutas → **crea/migra la SQLite** → abre la conexión.

Detalle importante y no documentado hasta ahora: **`loadAppContext` no es de sólo
lectura**. Comandos que parecen inocuos (`doctor`, `inspect`, `logs`) crean
directorios en el home y materializan la base de datos.

Tres comandos lo evitan por completo: `init` (haría fallar el bootstrap inicial),
`plan` (por diseño, sin efectos) y `service` (sólo necesita las rutas).

### 3.2 Rutas XDG

`internal/config/paths.go:20-53`. En Linux:

| Ruta | Contenido |
|---|---|
| `~/.config/devherd/config.json` | Configuración global (escritura atómica, modo `0600`) |
| `~/.local/share/devherd/devherd.db` | SQLite principal: proyectos y dominios |
| `~/.local/share/devherd/observability/devherd-observe.db` | SQLite **separada** de Observe |
| `~/.local/share/devherd/local_proxy/` | Assets del proxy externo (compose, Caddyfile, .env) |
| `~/.local/share/devherd/proxy/Caddyfile` | Caddyfile del driver en host |
| `~/.local/share/devherd/compose/shared-services/` | Compose de Redis y Mailpit |
| `~/.local/state/devherd/logs/` | Reservado |

En macOS y Windows el "data root" cae en `os.UserConfigDir()` y el "state root" en
`os.UserCacheDir()` (`paths.go:73-89`), pero `XDG_DATA_HOME`/`XDG_STATE_HOME`
mantienen prioridad si están definidas.

### 3.3 Persistencia

**Dos bases SQLite independientes**, con dos sistemas de esquema distintos:

- **Principal** (`internal/database/`): migraciones **versionadas** con tabla
  `schema_migrations` y archivos numerados embebidos (`migrations/0001_init.sql`).
  El runner aplica sólo lo que falta (`db.go:49-86`) y tolera bases legacy
  registrando la baseline (test en `migrations_test.go:66`).
- **Observe** (`internal/observe/`): **sin versionado**. `migrations.go` son 6
  líneas con un `//go:embed schema.sql`, y `Ensure` reejecuta el esquema completo
  en cada invocación. Es idempotente sólo porque todo es `CREATE ... IF NOT EXISTS`.

Contrato a respetar en futuras migraciones: `migrate()` **no envuelve cada
migración en una transacción** (`db.go:68-83`), así que el SQL debe ser idempotente.

De las 8 tablas de la base principal, **sólo 3 tienen código Go que las use**
(`parks`, `projects`, `project_domains`). `settings`, `runtime_preferences`,
`services`, `sentry_configs` y `events` están declaradas y vacías de uso.

### 3.4 Ejecución de comandos externos

`internal/runner/` define el seam de ejecución: interfaz `Runner`, implementación
`Cmd` con timeout opcional, trazas `slog.Debug` y captura combinada de stdout+stderr.
Cobertura **100%**.

Pero **la unificación está a medias**: sólo `compose` y `services` lo usan. Siguen
existiendo helpers `exec` propios en `doctor` (`doctor.go:464`, `:607`), `preflight`
(`preflight.go:531`), `proxy` (`external.go:533`, `caddy.go:153`), `dns`
(`hosts.go:127`), `observe` (`docker.go:122`) y `compose/logs` (`logs.go:39`) — la
duplicación exacta que la documentación del paquete afirma haber eliminado
(`runner.go:1-4`).

Consecuencia observable: **timeouts inconsistentes** — 3 s en doctor y preflight,
5 s en observe, 10 s en proxy, ninguno en compose.

### 3.5 Logging

Flags globales `--verbose` (nivel DEBUG) y `--log-json` (handler JSON), aplicados en
`PersistentPreRunE` (`root.go:24-27`, `logging.go:18-34`).

El contrato es claro y se respeta en todo el código: **diagnóstico a stderr vía
`slog`, salida de producto a stdout vía `cmd.OutOrStdout()`**. Eso permite, por
ejemplo, `devherd list --json > proyectos.json 2> diagnostico.log`.

---

## 4. Ciclo de vida de un proyecto Compose

### 4.1 Resolución

`compose.ResolveProject` (`internal/compose/project.go:67-109`):

1. Si existe `.devherd.yml`, manda el manifiesto (`Source = "manifest"`).
2. Si no, autodetecta el primer archivo de `docker-compose.yml`, `docker-compose.yaml`,
   `compose.yml`, `compose.yaml` (`Source = "autodetect"`).
3. Si no hay ninguno, error.

**No busca hacia arriba en el árbol**: ejecutar `devherd up` desde un subdirectorio
del proyecto falla.

### 4.2 Nombre de proyecto estable

`ProjectNameForPath` (`project.go:335-352`) genera `devherd-<slug>-<sha1[:8]>` a
partir de la **ruta absoluta**. Dos clones del mismo repo en directorios distintos
obtienen nombres Compose distintos, que es exactamente lo que permite tenerlos
levantados a la vez.

`LegacyProjectNameForPath` conserva el esquema antiguo (sólo el basename). `down` y
`stop` ejecutan **una segunda pasada** con ese nombre para limpiar stacks creados
antes del cambio (`project.go:152-168`), lo que duplica su tiempo de ejecución en la
práctica totalidad de los proyectos.

### 4.3 El manifiesto `.devherd.yml`

Esquema real según `project.go:44-59`. Se deserializa **sin `KnownFields`**: las
claves desconocidas se ignoran en silencio.

```yaml
version: 1                       # se parsea, nunca se valida ni se usa

compose:                         # OBLIGATORIO
  files:                         # obligatorio y no vacío; rutas relativas existentes
    - docker-compose.yml
  env_file: .env                 # opcional; se pasa como --env-file

proxy:                           # opcional
  domain: mi-app.localhost       # máxima precedencia sobre la DB y <name>.<tld>
  service: web                   # servicio compose que recibe el tráfico
  port: 8080                     # puerto INTERNO del contenedor
```

Reglas que conviene conocer:

- `compose.files` vacío es **error duro**, no un fallback a autodetección.
- Los archivos referenciados **deben existir ya** en el momento de resolver.
- `proxy.service` y `proxy.port` sólo surten efecto **juntos**; si falta uno, se cae
  al `switch` por framework, que sólo conoce `vue+flask`.
- Declarar `env_file` **desactiva** la carga automática de `<root>/.env` por parte de
  docker compose.

### 4.4 Overrides encadenados

Al levantar, `prepareComposeProject` (`internal/cli/compose_runtime.go:15-36`) puede
añadir dos archivos `-f` adicionales:

| Archivo | Lo escribe | Lo borra |
|---|---|---|
| `.devherd.proxy.override.yml` | `proxy apply` / `up` en modo externo | `down` |
| `.devherd.observe.override.yml` | `observe attach` | `observe detach` |

Ambos se escriben **dentro del repositorio del usuario**. Ninguno se añade al
`.gitignore` automáticamente; sólo llevan un comentario de cabecera.

Divergencia relevante: **`plan` e `inspect` no incluyen los overrides** porque no
cargan el contexto de aplicación. El comando que muestra `devherd plan` no es
necesariamente el que ejecuta `devherd up`.

---

## 5. Proxy y DNS

### 5.1 `caddy-docker-external` (recomendado)

DevHerd administra un contenedor Caddy (`infra_caddy`) en una red Docker compartida
(`infra_web`), con sus assets bajo `~/.local/share/devherd/local_proxy/`.

`proxy apply` en este modo hace, en orden (`internal/cli/proxy.go:64-96`):

1. `BuildExternalProject` — calcula dominio efectivo, prefijo y rutas.
2. `EnsureComposeOverride` — escribe el override que conecta los servicios a
   `infra_web` con alias `<prefijo>-<servicio>`.
3. `ConnectProject` — además, conecta imperativamente cada contenedor con
   `docker network connect --alias`.
4. `ApplyExternalProxy` — fusiona los bloques administrados en el Caddyfile, levanta
   el contenedor y ejecuta `caddy validate` + `caddy reload` dentro de él.
5. `syncManagedDomains` — sincroniza `/etc/hosts`.

El merge del Caddyfile usa **marcadores explícitos**
(`# devherd managed start <dominio>` / `... end`), con una segunda pasada de
fallback por conteo de llaves para migrar bloques del formato antiguo
(`external.go:389-459`).

Precedencia del dominio (`effectiveDomain`, `external.go:277-287`):
`.devherd.yml proxy.domain` → dominio en la base de datos → `<nombre>.<tld>`.

### 5.2 `caddy` en host (limitado)

`internal/proxy/caddy.go:119-146` mapea el framework a targets **hardcodeados en
loopback**: `vue+flask` → `127.0.0.1:8000` + `:5173`, `flask` → `:8000`,
`vue` → `:5173`. Cualquier otro framework es un error explícito.

Tres limitaciones que hay que tener presentes:

- **Ignora `.devherd.yml` por completo.** `proxy.service`/`port`/`domain` sólo
  funcionan en modo externo. Un Laravel generado por `scaffold` no se puede publicar
  con este driver.
- **Regenera el Caddyfile entero**, así que `proxy apply <un-proyecto>` borra los
  sitios de los demás — mientras que `/etc/hosts` sí conserva todos los dominios.
- Recarga contra el admin `127.0.0.1:2020` (`caddy.go:18`), no el `:2019` por
  defecto de Caddy. Si hay un Caddy de sistema corriendo, el reload falla y DevHerd
  arranca un **segundo** Caddy que competirá por el puerto 80.

### 5.3 DNS

`dns.SyncHosts` (`internal/dns/hosts.go:46-84`) reescribe un bloque delimitado por
`# devherd start` / `# devherd end`, preservando el resto de `/etc/hosts`. Valida
cada dominio contra una regex estricta antes de escribir (defensa anti-inyección,
`hosts.go:21-44`), avisa por stderr antes de pedir `sudo`, y es idempotente.

Puntos a conocer:

- **Siempre pide `sudo`**, incluso si el contenido resultante es idéntico.
- **También se ejecuta en modo externo**, pese a que el argumento de venta de ese
  driver es "sin sudo". Con `.localhost` es innecesario en glibc moderno.
- **Sólo IPv4** (`127.0.0.1`); no emite entrada `::1`.
- **Nada limpia nunca el bloque**: los dominios permanecen aunque se elimine el
  proyecto.
- `proxy apply <proyecto>` sincroniza **todos** los dominios registrados, no sólo el
  filtrado (`proxy.go:87`, `:109-112`).

---

## 6. Detección de proyectos

`detector.Discover` (`internal/detector/detector.go:31-78`) escanea el directorio
raíz y **sólo un nivel** de hijos directos, ignorando `node_modules` y ocultos.

Señales reconocidas (`inspectDirectory`, `detector.go:145-214`): `artisan` +
`composer.json` → Laravel; `package.json` → Node (Vue si declara la dependencia
`vue`); `requirements.txt`/`pyproject.toml` → Python (Flask por substring);
`go.mod` → Go; archivos compose o `Dockerfile` → Docker (sólo en la raíz).

Limitaciones reales del catálogo:

- No reconoce **React, Next, Angular, Svelte, Nuxt, Django, FastAPI, Rails, Spring,
  Rust ni .NET**.
- **PHP sin `artisan` es invisible**: Symfony y WordPress no se detectan.
- `flask` se marca por **substring** en el archivo completo: `flask-caching` o
  incluso un comentario que mencione flask lo activan.
- Las features de los hijos **se acumulan en el padre**, lo que produce
  clasificaciones sintéticas: un repo con `frontend/` Vue y `backend/` Flask se
  reporta como framework `vue+flask` en la raíz.
- **El detector no detecta puertos**; eso vive en `scaffold`.

---

## 7. Scaffold

Genera `docker-compose.devherd.yml` y `.devherd.yml` para repos sin contenedores
(nombre propio deliberado, para no pisar un compose del usuario).

Stacks soportados (`scaffold.go:54-90`): combo `vue+flask`, `laravel`, `vue`,
`flask`, `node`, `go`. Bases de datos opcionales: MySQL, MariaDB, Postgres, MongoDB.

**Laravel es el único stack "completo"** (`plan.Complete`): deriva la base de datos y
las credenciales del `.env` del propio repo, añade Redis y, si hay `package.json`,
un servicio Vite. En ese camino **`--db` y `--redis` se ignoran silenciosamente**.

El comando del contenedor Laravel (`laravelCommand`, `scaffold.go:226-250`) es una
cadena idempotente que instala extensiones, Composer, copia `.env`, genera `APP_KEY`,
espera a la base de datos y arranca `artisan serve`. Los cuatro commits recientes de
la rama van justo sobre esto: idempotencia al reiniciar, escape de `$` para que
docker compose no interpole (`composeEscape`, `scaffold.go:560`), y Vite en modo
`build --watch` en vez de dev server (para que los assets salgan compilados).

Bordes ásperos que conviene documentar como limitaciones, no como features:

- **`hasVite` sólo comprueba que exista `package.json`** (`scaffold.go:309-313`):
  cualquier Laravel con dependencias JS recibe un servicio `vite` aunque no use Vite,
  y ese contenedor termina en `Exited` sin aviso claro.
- **El bucle `until php artisan migrate --force` es infinito** si la migración falla
  por algo distinto a "la base aún no está lista" (SQL inválido, credenciales malas).
  El contenedor nunca llega a `artisan serve` y `up` parece colgado.
- **Laravel siempre añade Redis**, sin forma de desactivarlo.
- `DB_CONNECTION` vacío o desconocido cae en **MySQL** por defecto.
- **El comando reinstala extensiones en cada arranque**: sin Dockerfile no hay capa
  cacheada.
- Los puertos `8000`/`5173` del combo `vue+flask` son **fijos por contrato** con el
  proxy; cambiarlos en el compose generado rompe el enrutado en silencio.
- Si `freePort` agota sus 200 intentos, devuelve 0 y el bloque `ports` **se omite sin
  ningún mensaje**.

---

## 8. Observabilidad

### 8.1 El collector

Servidor HTTP en `127.0.0.1:9777` por defecto (`internal/observe/server.go:17`),
**en foreground**: no hay daemon, ni pidfile, ni `observe stop`. Se apaga con Ctrl+C,
con shutdown ordenado del poller vía `WaitGroup` (`server.go:83-98`).

Cuatro patrones registrados (`server.go:44-51`): `/health`, `/api/observe/` (API del
panel), `/api/` (ingesta, sólo POST) y `/` (panel).

Rutas de ingesta aceptadas (`parseAPIPath`, `server.go:217-232`):
`/api/<proyecto>/event`, `/api/<proyecto>/store` y `/api/<proyecto>/envelope`.
Cuerpo limitado a 2 MiB. Respuesta `202 Accepted` con un JSON propietario.

Un poller consulta Docker cada 10 s para snapshotear contenedores etiquetados.

**Un proyecto llamado `observe` es un nombre reservado de facto**: colisiona con el
patrón `/api/observe/` del panel y su ingesta devuelve 405.

### 8.2 Compatibilidad Sentry: parcial

El DSN generado es `http://devherd@127.0.0.1:9777/<proyecto>`
(`internal/cli/observe.go:832-835`), con clave pública literal `devherd` y el
**nombre** del proyecto como ID.

El parser de envelopes (`server.go:234-271`) tiene limitaciones que probablemente
impiden que un SDK oficial funcione tal cual:

- **No descomprime gzip**, que es el default de varios SDKs (Python, PHP, Node).
  Este es el bloqueador número uno.
- **Ignora el campo `length`** del header de ítem, la forma canónica de delimitar
  payloads.
- Asume **una línea por payload**; un payload multilínea desincroniza el parser.
- **Ignora `X-Sentry-Auth`** y `?sentry_key=`: no hay validación de DSN.
- El **array `fingerprint`** que envían los SDKs se descarta.

### 8.3 Agrupación en issues

`Fingerprint` (`event.go:82-92`) es un SHA-1 de cinco componentes: proyecto, tipo de
excepción, mensaje normalizado, culprit y servicio.

La normalización del mensaje (`event.go:124-131`) sólo hace trim, minúsculas y
colapso de espacios. **No enmascara números, UUIDs, rutas ni IDs**, así que
`"user 42 not found"` y `"user 43 not found"` generan dos issues distintos. Es el
principal limitante práctico del agrupamiento.

Dos detalles con consecuencias:

- El fingerprint se calcula **antes** del enriquecimiento por Docker
  (`server.go:145` frente a `:154`), de modo que el `service` inferido del contenedor
  no participa en la agrupación. Dos eventos idénticos pueden separarse según si el
  SDK envió `service` explícitamente.
- **Los estados de issue `seen`/`resolved`/`ignored` no existen** como funcionalidad:
  no hay comando ni endpoint que los cambie. Todo issue queda en `new` para siempre.

### 8.4 Alertas

Cuatro tipos: `new-issue`, `error-rate`, `container-exit`, `container-restart`.
Se evalúan **inline**, dentro de la transacción de escritura del evento; no hay motor
ni scheduler.

Una "delivery" es **exclusivamente una fila en SQLite** (`store.go:904-914`). No hay
webhooks, ni comandos, ni notificaciones del sistema operativo, ni email. Se leen con
`observe alert deliveries` o en el panel.

`error-rate` **no tiene cooldown ni deduplicación**: una vez superado el umbral, cada
evento posterior dentro de la ventana genera otra delivery.

### 8.5 `devherd sentry` es un placeholder

`internal/cli/sentry.go` son 92 líneas que **no importan ningún paquete interno**.
`sentry init --dry-run` imprime un plan estático y hardcodeado; sin `--dry-run`
devuelve "no implementado". `set-dsn` y `test` están ocultos (`Hidden: true`) y
también devuelven "no implementado".

`internal/sentry/` es un `doc.go` de 2 líneas, `templates/sentry/` un README
placeholder no embebido, y la tabla `sentry_configs` no la lee ni escribe nadie.

**La funcionalidad tipo-Sentry real y usable vive íntegramente en `devherd observe`**,
y ambos subsistemas están completamente desconectados entre sí.

### 8.6 Retención

`observe cleanup --days N` (default 14) borra por fecha de `container_logs`,
`container_events`, `alert_deliveries`, `events` e `issues`.

**No borra `containers`**: los snapshots de contenedores no caducan nunca, así que
contenedores eliminados hace meses siguen apareciendo en `observe containers` y en el
panel. Es la fuga de datos más visible.

Además, `events` se filtra por `timestamp` (**controlado por el cliente**), no por
`created_at`: un SDK con el reloj desfasado nunca se limpia. No hay `VACUUM`, así que
el archivo no se encoge, y no hay limpieza automática: es 100% manual.

---

## 9. Estado de ingeniería (medido)

### 9.1 Build y tests

Salidas reales sobre `76a8a24`:

```
$ go build ./...     → exit 0, sin salida
$ go vet ./...       → exit 0, sin salida
$ go test ./...      → todos los paquetes OK, ningún FAIL
Cobertura total: 41.3%
```

| Paquete | LOC | Cobertura |
|---|---|---|
| `internal/runner` | 59 | **100.0%** |
| `internal/version` | 21 | **100.0%** |
| `internal/scaffold` | 783 | 90.5% |
| `internal/detector` | 379 | 76.1% |
| `internal/database` | 478 | 68.3% |
| `internal/services` | 159 | 67.4% |
| `internal/observe` | 2388 | 54.7% |
| `internal/compose` | 448 | 51.6% |
| `internal/proxy` | 882 | 48.2% |
| `internal/dns` | 134 | 44.1% |
| `internal/config` | 224 | 40.3% |
| `internal/doctor` | 711 | 39.2% |
| `internal/preflight` | 602 | 38.4% |
| `internal/cli` | 2722 | **6.0%** |
| `cmd/devherd` | 15 | 0.0% |

**El riesgo se concentra en `internal/cli`**: 2722 líneas al 6%, la superficie más
grande y menos verificada del proyecto. Sus tests cubren helpers puros (`logging`,
`naming`, `open`, `serve`, `observe`, `proxy`, `init`); ningún comando se ejercita
end-to-end.

Otros puntos ciegos concretos: `config.Store` (Load/Save) al 0% pese a ser el punto
de persistencia de la configuración global; `preflight.inspectPorts` y
`inspectLaravelEnv` al 0%; los agregadores de `doctor.Report` al 0%.

### 9.2 Pipeline

| Pieza | Estado |
|---|---|
| `Makefile` | 10 targets, inyecta `Version/Commit/Date` por `-ldflags` |
| CI (`.github/workflows/ci.yml`) | `go vet` + `make build` + `go test -race` + cobertura + `golangci-lint` |
| Linter (`.golangci.yml`) | `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`, `gocritic`, `revive` |
| `Dockerfile` | Multi-stage, base distroless `nonroot`, `CGO_ENABLED=0` |
| `.goreleaser.yml` | 4 binarios (linux/darwin × amd64/arm64) + `.deb` + checksums |
| Instalación | `scripts/install-ubuntu.sh` compila desde fuente |

Tres huecos concretos:

1. **El release nunca se ha ejecutado.** GoReleaser está completo, pero ningún
   workflow lo dispara, no hay target `make release` y **no hay tags git**. Hoy
   `make build` produce como versión el hash corto de `git describe --always`.
2. **`install-ubuntu.sh` no inyecta ldflags**, así que el binario instalado por la
   ruta oficial reporta siempre `0.1.0-alpha / dev / unknown`.
3. **CI sólo corre en ubuntu-latest**, aunque hay código específico para darwin y
   windows en `paths.go` y `doctor.go`, validado únicamente con tests de tabla.

### 9.3 Código muerto

Confirmado por inspección:

- **Cuatro paquetes fantasma** de 2 líneas: `internal/api`, `internal/logs`,
  `internal/runtimes`, `internal/sentry`. Los dos últimos inducen a error, porque su
  funcionalidad ya existe implementada en otro sitio (`internal/compose/logs.go` y
  `internal/cli/sentry.go`).
- **Tres templates no embebidos ni referenciados**: `templates/caddy/Caddyfile.tmpl`
  (duplicado casi byte a byte del real, `internal/proxy/Caddyfile.tmpl`),
  `templates/nginx/server.conf.tmpl` (no hay driver nginx) y
  `templates/docker/docker-compose.base.yml` (superado por
  `internal/services/shared-services.compose.yml`).
- **Cinco tablas SQLite** declaradas y sin uso (§3.3).
- `LogsOptions.Services` existe en la API pero **ningún flag de la CLI lo rellena**.

---

## 10. Riesgos y deuda priorizados

Ordenados por relación impacto/esfuerzo.

### Alto impacto

1. **Cobertura de `internal/cli` (6% sobre 2722 LOC).** Es donde vive la orquestación
   real y donde más rompe un refactor. Necesita poder inyectar `appContext` para
   testear comandos end-to-end.
2. **Gzip en el endpoint de envelope.** Sin eso, la promesa de "compatible con SDKs
   Sentry" no se sostiene para Python, PHP ni Node.
3. **El Caddyfile externo se escribe antes de validarse** (`external.go:219` frente a
   `:228`). Si `caddy validate` falla, el archivo queda roto en disco sin rollback y
   el siguiente `apply` parte de ese estado.
4. **`prepareComposeProject` traga el error** de `resolveExternalProject` y devuelve
   el proyecto sin override de proxy **y sin el de observe**
   (`compose_runtime.go:26-28`), divergiendo en silencio de lo que `proxy apply`
   espera.

### Impacto medio

5. **Timeout de 10 s en todos los comandos Docker del proxy**, incluido el
   `docker compose up -d` del local_proxy: el primer arranque, que hace pull de la
   imagen de Caddy, agota el plazo.
6. **`RemoveExternalProxy` puede arrancar el proxy**: llama a
   `ensureExternalProxyReady`, de modo que `devherd down` es capaz de **levantar**
   `infra_caddy` si estaba apagado.
7. **`down` no limpia `/etc/hosts`** y **`stop` no borra el override de proxy**:
   ambos dejan residuos.
8. **`services.bootstrap()` sobrescribe el compose compartido incondicionalmente** en
   cada `service start|stop|status`, perdiendo cualquier edición del usuario — al
   contrario que el bootstrap del proxy, que sí respeta lo existente.
9. **Dos detectores de puertos con semánticas distintas**: `doctor` usa
   `/proc/net/tcp` + `lsof`; `preflight` usa `net.Listen` sobre loopback. Pueden dar
   veredictos divergentes para el mismo puerto.
10. **`internal/runner` sin adoptar** en 7 paquetes (§3.4).

### Impacto bajo / higiene

11. Release manual y sin tags (§9.2).
12. Paquetes fantasma, templates muertos y tablas huérfanas (§9.3).
13. `config.Default()` toca el filesystem y puede devolver una ruta relativa en el
    peor caso (`config.go:91`).
14. Defaults duplicados en tres sitios (`config.Default`, `ApplyPathDefaults`,
    `externalSettings`): los literales `infra_web` e `infra_caddy` se repiten.
15. `cfg.Proxy.HTTPSPort`, `cfg.DNS.Driver`, `cfg.DNS.ResolverIP` y
    `cfg.DNS.ManagedSuffix` son campos de configuración **inertes**.
16. `ensureComposeOrScaffold` decide comparando el **texto** del error
    (`strings.Contains(err.Error(), "no supported compose file")`): reformular ese
    mensaje rompe la integración sin fallar la compilación.

---

## 11. Fortalezas que conviene preservar

No todo es deuda. Estas decisiones están bien tomadas y merece la pena mantenerlas
como invariantes:

- **Separación de capas real**: `internal/cli` orquesta, los paquetes de dominio
  deciden. Ningún ciclo de importación.
- **Nombre de proyecto derivado de la ruta absoluta**: resuelve de raíz el problema
  de clones homónimos.
- **SQLite sin CGO**: habilita cross-compilación, distroless y CI trivial.
- **Plantillas embebidas** (`go:embed`): el binario es autocontenido.
- **Marcadores administrados explícitos** en `/etc/hosts` y en el Caddyfile, con
  fallback de migración desde el formato antiguo.
- **Validación estricta de dominios antes de tocar `/etc/hosts`**, con aviso previo
  de `sudo`.
- **Separación stdout/stderr** disciplinada entre producto y diagnóstico.
- **Bases de datos separadas** para plataforma y observabilidad: el volumen de
  eventos no compromete los datos de proyectos.
- **Escritura atómica de la configuración** con permisos `0600`.

---

## 12. Siguiente bloque recomendado

1. **Testabilidad de `internal/cli`** — inyectar `appContext` y cubrir los comandos
   con más efectos (`up`, `proxy apply`, `down`).
2. **Cerrar la compatibilidad Sentry** — gzip y respeto del campo `length`, o bien
   documentar explícitamente que el collector sólo acepta el endpoint propio.
3. **Robustecer el proxy externo** — validar antes de escribir, subir el timeout del
   primer arranque, no levantar el proxy desde `down`.
4. **Completar la adopción de `internal/runner`** en los 7 paquetes restantes, con lo
   que los timeouts quedan unificados y sube la cobertura de los paquetes que hoy
   sólo se pueden probar con Docker real.
5. **Automatizar el release** — tag + workflow de GoReleaser, y que
   `install-ubuntu.sh` inyecte ldflags o descargue el `.deb`.
6. **Limpiar el código muerto** — cuatro paquetes fantasma, tres templates y las
   tablas sin uso.
