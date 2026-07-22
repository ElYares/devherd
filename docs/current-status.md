# DevHerd: Estado Actual del Proyecto

Que existe hoy, que esta medido y que sigue pendiente. Ultima revision: **2026-07-21**
(commit `76a8a24`).

> Las metricas de esta pagina son salidas reales de comandos, no estimaciones. Para el
> analisis completo del sistema ver [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md); para usar la
> herramienta, [USAGE.md](USAGE.md).

## 1. Comandos disponibles

18 comandos de primer nivel, 36 ejecutables visibles contando subcomandos (38 registrados:
`sentry set-dsn` y `sentry test` estan marcados `Hidden` y no aparecen en la ayuda). Todos
implementados salvo donde se indica.

**Plataforma y proyectos**

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list [--json]`
- `devherd domain set <project> --domain <name>`
- `devherd plan [path]`
- `devherd inspect [path]`
- `devherd scaffold [path]`
- `devherd up [path]`
- `devherd serve [path]`
- `devherd stop [path]`
- `devherd down [path]`
- `devherd open <project>`
- `devherd logs [path]`

**Proxy y servicios**

- `devherd proxy apply [project]`
- `devherd proxy bootstrap`
- `devherd service start|stop|status [service]`

**Observabilidad**

- `devherd observe start|status|open|dsn`
- `devherd observe attach|detach`
- `devherd observe scan|containers|issues|events|timeline`
- `devherd observe alert add|list|remove|deliveries`
- `devherd observe cleanup`

**Placeholder (no funcional)**

- `devherd sentry init` — solo `--dry-run`, imprime un plan estatico
- `devherd sentry set-dsn` — oculto, devuelve `not implemented`
- `devherd sentry test` — oculto, devuelve `not implemented`

## 2. Que hace hoy DevHerd

### Inicializacion y estado local

- Crea configuracion bajo XDG y una base SQLite con **migraciones versionadas**
  (`schema_migrations` + archivos numerados embebidos).
- Guarda preferencias iniciales: driver de proxy, TLD y gestor de runtimes.
- Usa una **segunda base SQLite separada** para observabilidad.

### Diagnostico del host

- Valida rutas locales, Docker CLI, daemon Docker, modo del engine y `docker compose`.
- Adapta los chequeos al driver de proxy configurado.
- Funciona **sin** `devherd init`, cayendo a la configuracion por defecto.

### Registro y deteccion de proyectos

- Registra directorios con `park` y detecta proyectos en la carpeta y **un nivel** de
  hijos.
- Reconoce Laravel, Node/Vue, Python/Flask, Go y Docker.
- Elimina del registro los proyectos detectados que ya no existen en disco.

### Proyectos Docker Compose

- `plan` inspecciona sin efectos; `inspect` audita colisiones; `up` ejecuta preflight
  automatico; `stop`, `down` y `logs` completan el ciclo.
- Nombre de proyecto Compose **estable por ruta absoluta** (`devherd-<slug>-<hash>`), con
  segunda pasada de limpieza para stacks creados con el esquema antiguo.
- Manifiesto `.devherd.yml` con `compose.files`, `compose.env_file`, `proxy.domain`,
  `proxy.service` y `proxy.port`.

### Generacion de compose (`scaffold`)

- Genera `docker-compose.devherd.yml` y `.devherd.yml` para repos sin contenedores.
- Stacks: combo `vue+flask`, `laravel`, `vue`, `flask`, `node`, `go`.
- Bases de datos opcionales (MySQL, MariaDB, Postgres, MongoDB) y Redis.
- **Laravel es el stack completo**: deriva base de datos y credenciales del `.env` del
  repo, anade Redis siempre y, si hay `package.json`, un servicio Vite en modo
  `build --watch`.
- Elige puertos de host libres para no colisionar con proyectos ya levantados.
- `up` lo ofrece automaticamente cuando el proyecto no tiene compose.

### Proxy local

- Driver `caddy-docker-external` (recomendado): contenedor Caddy administrado, override
  compose que conecta los servicios a `infra_web` con aliases estables, merge del
  Caddyfile con **marcadores explicitos** y recarga dentro del contenedor.
- Driver `caddy` en host: renderiza el Caddyfile completo y recarga con `sudo`.
- `proxy bootstrap` crea o repara los assets del proxy externo.
- Sincronizacion de `/etc/hosts` en un bloque administrado, con validacion estricta de
  dominios y aviso previo de `sudo`.

### Servicios compartidos

- Redis y Mailpit en la red `infra_net`, publicados en loopback.

### Observabilidad (`observe`)

- Collector HTTP local en foreground con panel web y base SQLite propia.
- Ingesta por endpoint propio, endpoint `store` y un parser de envelopes tipo Sentry.
- Normalizacion de eventos y agrupacion en issues por fingerprint SHA-1.
- Datos fuera del modelo normalizado (`context`, `tags`, breadcrumbs) preservados en
  `raw_payload` y expuestos en `observe timeline` y en la API del panel.
- Correlacion con contenedores Docker por labels `devherd.*`, con captura de logs
  cercanos al fallo y timeline por evento.
- Alertas locales (`new-issue`, `error-rate`, `container-exit`, `container-restart`)
  registradas en la base local.
- Limpieza manual de datos antiguos.

## 3. Estado de ingenieria (medido)

Sobre el commit `76a8a24`:

```
go build ./...   → OK
go vet ./...     → OK
go test ./...    → todos los paquetes pasan
Cobertura total  → 41.3%
```

| Paquete | Cobertura |
|---|---|
| `internal/runner` | 100.0% |
| `internal/version` | 100.0% |
| `internal/scaffold` | 90.5% |
| `internal/detector` | 76.1% |
| `internal/database` | 68.3% |
| `internal/services` | 67.4% |
| `internal/observe` | 54.7% |
| `internal/compose` | 51.6% |
| `internal/proxy` | 48.2% |
| `internal/dns` | 44.1% |
| `internal/config` | 40.3% |
| `internal/doctor` | 39.2% |
| `internal/preflight` | 38.4% |
| `internal/cli` | **6.0%** |

Infraestructura disponible: `Makefile` con inyeccion de version por `ldflags`, CI en
GitHub Actions (`vet` + build + `test -race` + cobertura + `golangci-lint`), linter
configurado, `Dockerfile` multi-stage sobre distroless y `.goreleaser.yml` con paquete
`.deb`.

## 4. Limitaciones actuales

### Cobertura y verificacion

- `internal/cli` concentra 2722 lineas al 6% de cobertura: es la superficie mas grande y
  menos verificada. Sus tests cubren helpers, no comandos end-to-end.
- El CI solo corre en `ubuntu-latest`, pese a que hay codigo especifico de macOS y Windows.

### Proxy

- El driver `caddy` en host solo enruta `vue+flask`, `flask` y `vue`, con puertos fijos en
  loopback, e **ignora la metadata `proxy` del manifiesto**. Un Laravel generado por
  `scaffold` no se puede publicar con ese driver.
- En modo host, `proxy apply <proyecto>` regenera el Caddyfile completo y borra los sitios
  de los demas proyectos.
- El valor `nginx` se acepta en `init --proxy` pero **no existe driver nginx**.
- El Caddyfile externo se escribe **antes** de validarse: si `caddy validate` falla, queda
  en disco un archivo roto sin rollback.
- Timeout fijo de 10 s para todos los comandos Docker del proxy, incluido el primer
  arranque del contenedor (que hace pull de la imagen).
- `down` puede **arrancar** el contenedor del proxy si estaba apagado.

### DNS

- `SyncHosts` siempre pide `sudo`, incluso si el contenido no cambia, y tambien en modo
  externo (donde con `.localhost` suele ser innecesario).
- Solo escribe entradas IPv4; no hay `::1`.
- **Nada limpia el bloque**: los dominios permanecen en `/etc/hosts` tras eliminar el
  proyecto.

### Ciclo de vida

- `stop` no borra el override de proxy; `down` no limpia `/etc/hosts`.
- `plan` e `inspect` no incluyen los overrides, asi que el comando que muestra `plan` puede
  diferir del que ejecuta `up`.
- `ResolveProject` no busca hacia arriba: hay que ejecutar los comandos desde la raiz del
  proyecto o pasar la ruta.

### Deteccion

- Solo un nivel de profundidad y catalogo estrecho: no reconoce React, Next, Angular,
  Svelte, Nuxt, Django, FastAPI, Rails, Spring, Rust ni .NET. PHP sin `artisan` es
  invisible.
- Las features de los subdirectorios se acumulan en el padre, produciendo clasificaciones
  sinteticas como `vue+flask`.

### Scaffold

- `hasVite` solo comprueba que exista `package.json`: cualquier Laravel con dependencias
  JS recibe un servicio Vite aunque no lo use.
- El bucle de espera de la base de datos (`until php artisan migrate`) es infinito si la
  migracion falla por un motivo distinto a "la base aun no esta lista".
- El comando de Laravel reinstala extensiones en cada arranque: sin Dockerfile no hay capa
  cacheada.
- En Laravel, `--db` y `--redis` se ignoran silenciosamente.

### Observabilidad

- El parser de envelopes **no descomprime gzip**, que es el default de varios SDKs
  oficiales. Es el principal bloqueador para usar un SDK Sentry real contra el collector.
- El fingerprint no enmascara numeros ni identificadores, asi que mensajes que solo
  difieren en un ID generan issues distintos.
- Los estados de issue `seen`/`resolved`/`ignored` **no existen**: no hay comando ni
  endpoint que los cambie.
- `observe cleanup` **no borra el inventario de contenedores**, que crece sin limite.
- Las alertas solo escriben una fila en la base local: no hay webhooks ni notificaciones.
- `error-rate` no tiene periodo de enfriamiento.
- El collector no tiene autenticacion; el default es loopback.

### Distribucion

- GoReleaser esta configurado (4 binarios + `.deb`) pero **ningun workflow lo dispara** y
  no hay tags git: el release nunca se ha ejecutado.
- `scripts/install-ubuntu.sh` no inyecta los metadatos de version.

### Codigo muerto

- Cuatro paquetes de dos lineas sin implementacion: `internal/api`, `internal/logs`,
  `internal/runtimes`, `internal/sentry` (los dos ultimos inducen a error, porque su
  funcionalidad ya vive en otro sitio).
- Tres plantillas no embebidas ni referenciadas: `templates/caddy/`, `templates/nginx/`,
  `templates/docker/`.
- Cinco tablas SQLite declaradas y sin uso: `settings`, `runtime_preferences`, `services`,
  `sentry_configs`, `events`.

## 5. Siguiente bloque recomendado

1. Testabilidad de `internal/cli`: inyectar `appContext` y cubrir los comandos con mas
   efectos (`up`, `proxy apply`, `down`).
2. Cerrar la compatibilidad Sentry (gzip y campo `length`) o documentar explicitamente que
   el collector solo acepta el endpoint propio.
3. Robustecer el proxy externo: validar antes de escribir, ampliar el timeout del primer
   arranque y no levantar el proxy desde `down`.
4. Completar la adopcion de `internal/runner` en los siete paquetes que aun ejecutan
   comandos por su cuenta, unificando timeouts.
5. Automatizar el release (tag + workflow de GoReleaser).
6. Ampliar el routing del proxy en host, o documentarlo formalmente como driver de alcance
   reducido frente a `caddy-docker-external`.

---

# Historial de validaciones

Registro de lo que se valido en entornos reales, conservado por trazabilidad. Las rutas y
nombres de usuario corresponden a las maquinas donde se hicieron esas pruebas.

## Validacion inicial (proyecto de ejemplo)

Sobre `/home/elyarestark/develop/examples/hello-vue-flask-docker` se valido:
`init`, `doctor`, `park`, `list`, `domain set`, `up`, `down` y `sentry init --dry-run`
con stacks `python` y `node`.

Tambien se comprobo que el backend Flask responde en `127.0.0.1:8000`, el frontend Vite en
`127.0.0.1:5173`, que el proyecto se detecta como `vue+flask` y que el stack se registra
como `node+python+docker`.

Ademas se valido `plan` sobre dos proyectos reales: `aang-server` (resuelve
`docker-compose.yml` + `docker-compose.shared.yml` y detecta el `.env` local) y
`landing-page-neura` (se corrigio la autodeteccion para fijar `docker-compose.dev.yml`
como stack local canonico).

## Flujo manual con `local_proxy` (previo a la automatizacion)

Se comprobo a mano el flujo con red Docker compartida `infra_web`, aliases de red para
frontend y backend, regla manual en el `Caddyfile` y dominio `http://mi-demo.localhost`.
Quedo validado end-to-end que el dominio resuelve, que Caddy enruta `/` al frontend y
`/api/*` al backend, y que el frontend consume la API por el dominio del proxy.

El runtime actual ya no depende de esa ruta personal: `caddy-docker-external` usa
`proxy.external_dir` desde la config, con default portable bajo
`~/.local/share/devherd/local_proxy`.

## Automatizado en `2026-05-04`

Sobre la base del flujo manual anterior, DevHerd automatizo: usar `local_proxy` como
driver oficial, crear o reparar el proxy portable, usar `.localhost` como TLD por defecto
en ese modo, generar `.devherd.proxy.override.yml`, conectar servicios a `infra_web`, crear
aliases estables, escribir y refrescar los bloques de dominio en el Caddyfile externo,
recargar Caddy dentro del contenedor y resolver `open` contra el dominio del manifiesto.

## Stack sensible real (`2026-05-04`)

Sobre `/home/elyarestark/develop-work/aang-server` se valido en real el ciclo completo:
`init --proxy caddy-docker-external`, `doctor`, `plan`, `up`, `park`, `proxy apply`,
`open` y `down`.

Durante esa validacion se corrigieron dos huecos: `down` dejaba el bloque del dominio en
`local_proxy` junto con el override generado, y `park` podia detectar `node_modules` como
proyecto falso.

## Validacion operativa de `aang-server` y `Uniformes`

Con dos proyectos Laravel bajo `/home/elyares/develop/work` quedo validado que ambos
pueden estar arriba a la vez, que `http://aang.localhost` y `http://uniformes.localhost`
responden `200 OK`, que cada proyecto publica su propio bloque en `local_proxy` y que cada
uno mantiene su propia cookie de sesion y sus prefijos de cache/Redis.

`aang-server` conserva ademas su volumen MySQL legado mediante
`DB_VOLUME_NAME=aang-server_aang_db_data` y `DB_VOLUME_EXTERNAL=true`.

Se aplico en ambos el patron de aislamiento por `COMPOSE_NAME_PREFIX` documentado en
[USAGE.md, seccion 8](USAGE.md#8-patrones-recomendados-para-proyectos-compose).

## Validacion operativa pendiente

- Validar el mismo flujo sobre `poderygozo-landing-page`.
- Confirmar el entrypoint real de `RetailDataOps`.
