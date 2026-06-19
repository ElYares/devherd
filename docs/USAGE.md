# Guia de uso de DevHerd

Guia practica de usuario: instalacion, todos los comandos y flags (derivados del codigo
real), ejemplos y flujos de trabajo comunes.

> DevHerd esta en estado MVP/alpha y es **Ubuntu/Linux-first**. Requiere Docker (con
> `docker compose`) y, para el modo de proxy en host, el binario `caddy`.

## 1. Requisitos

DevHerd valida los prerequisitos con `devherd doctor`. Los basicos son:

- **Docker CLI** y daemon Docker corriendo (engine Linux).
- **docker compose** (plugin v2: `docker compose ...`).
- Capacidad de escritura en los directorios locales (`~/.config/devherd`,
  `~/.local/share/devherd`, `~/.local/state/devherd`).

Segun el driver de proxy elegido:

- Driver `caddy` (en host): binario `caddy` en PATH, puertos 80/443 libres, acceso `sudo`
  (para editar `/etc/hosts` y recargar Caddy). `dnsmasq` es opcional.
- Driver `caddy-docker-external`: solo Docker; DevHerd administra un contenedor Caddy
  ("local_proxy") y las redes Docker `infra_web` e `infra_net`.

## 2. Instalacion y build

### 2.1 Durante desarrollo (sin instalar)

Desde la raiz del repositorio:

```bash
go mod tidy
go run ./cmd/devherd --help
go run ./cmd/devherd <comando> [args...]
```

### 2.2 Instalar el binario (Ubuntu)

```bash
# Compila e instala en ~/.local/bin/devherd
./scripts/install-ubuntu.sh

# (Opcional) instala Caddy para el modo de proxy en host
./scripts/install-caddy-ubuntu.sh

devherd --help
```

`scripts/install-ubuntu.sh` ejecuta `go build -o ~/.local/bin/devherd ./cmd/devherd`.
Asegurate de tener `~/.local/bin` en tu `PATH`.

### 2.3 Build manual

El binario ya no se versiona en el repositorio (`/devherd` esta en `.gitignore`); se
compila localmente. La via recomendada es el `Makefile`, que inyecta los metadatos de
version (`version.Version/Commit/Date`) via `-ldflags`:

```bash
make build         # compila en bin/devherd con version + commit + fecha
./bin/devherd --help

make install       # go install en $GOBIN con los mismos metadatos
make run ARGS="doctor"   # compila y ejecuta
```

Targets disponibles del `Makefile`: `build`, `install`, `test` (con `-race`), `cover`,
`vet`, `lint` (golangci-lint), `tidy`, `run`, `clean` y `help` (lista todos los targets).

Tambien puedes compilar a mano sin metadatos de version:

```bash
go build -o devherd ./cmd/devherd
./devherd --help
```

### 2.4 Desinstalar

```bash
./scripts/uninstall.sh   # elimina ~/.local/bin/devherd
```

Los ejemplos a continuacion usan `devherd ...`. Si trabajas sin instalar, sustituye por
`go run ./cmd/devherd ...`.

## 3. Flags globales

- `--help` / `-h`: ayuda de cualquier comando o subcomando.
- `--version`: imprime la version enriquecida con commit y fecha de build, en el formato
  `Version (commit X, built Y)` (p. ej. `0.1.0-alpha (commit a1b2c3d, built 2026-06-19T...)`).
  Los valores reales de commit/fecha se inyectan al compilar con `make build`/`make install`;
  con un `go build` plano salen los defaults (`commit dev, built unknown`).
- `--verbose`: habilita logging de diagnostico a nivel DEBUG en **stderr** (ver mas abajo).
- `--log-json`: emite los logs de diagnostico como JSON en **stderr** (util para scripting
  o agregadores de logs).

Estos dos ultimos flags son globales (persistentes) y aplican a cualquier subcomando. El
logging de diagnostico se separa de la salida "de producto": los mensajes utiles al usuario
van a **stdout** y los diagnosticos a **stderr**, de modo que puedes redirigir cada flujo por
separado. Sin `--verbose` el nivel por defecto es INFO.

```bash
devherd --verbose up /ruta/al/proyecto
devherd --verbose --log-json observe start 2> devherd.log
```

Casi todos los comandos que operan sobre proyectos requieren haber ejecutado
`devherd init` antes; de lo contrario fallan con *"DevHerd is not initialized. Run
`devherd init` first"*.

## 4. Referencia de comandos

### 4.1 `devherd init`

Inicializa los directorios locales, la config (`config.json`) y la base SQLite. Si el
driver es `caddy-docker-external`, ademas crea los assets del proxy externo.

Flags (`internal/cli/init.go:100-102`):

| Flag | Default | Valores | Descripcion |
|------|---------|---------|-------------|
| `--proxy` | `caddy` | `caddy`, `nginx`, `caddy-docker-external` | Driver de proxy reverso. |
| `--tld` | `test` | cualquier TLD | Dominio de nivel superior local. |
| `--runtime-manager` | `mise` | `mise`, `asdf` | Gestor de runtimes. |

Notas:

- Si pasas `--proxy caddy-docker-external` sin `--tld`, el TLD por defecto pasa a
  `localhost` (`init.go:116-119`, `proxy.DefaultTLDForDriver`).
- `init` es seguro de re-ejecutar: reusa config existente y reporta el estado
  (`created`/`reused`, `migrated`).

```bash
devherd init
devherd init --proxy caddy-docker-external
devherd init --proxy caddy --tld test --runtime-manager mise
```

Salida tipica:

```
DevHerd initialized
config: /home/usuario/.config/devherd/config.json
database: /home/usuario/.local/share/devherd/devherd.db
proxy driver: caddy-docker-external
local tld: .localhost
runtime manager: mise
external proxy dir: /home/usuario/.local/share/devherd/local_proxy
...
config status: created
database status: created
```

### 4.2 `devherd doctor`

Valida los prerequisitos del host segun el driver configurado. Devuelve codigo de salida
distinto de 0 si hay fallos.

```bash
devherd doctor
```

Salida (formato `STATUS  NOMBRE  MENSAJE`):

```
OK    local paths      writable local directories ready at /home/usuario/.config/devherd
OK    Docker CLI       found at /usr/bin/docker
OK    Docker daemon    server 27.0.3
...
summary: 0 failure(s), 1 warning(s)
```

### 4.3 `devherd park [path]`

Registra un directorio para descubrimiento automatico de proyectos y los inserta/actualiza
en la base. Requiere una ruta de directorio (`cobra.ExactArgs(1)`, `internal/cli/park.go`).

```bash
devherd park /home/usuario/develop/examples
```

Imprime los proyectos detectados con columnas `NAME / FRAMEWORK / STACK / DOMAIN / PATH`.
El dominio principal se deriva del nombre del proyecto y el TLD configurado
(p. ej. `mi-app.test`).

### 4.4 `devherd list`

Lista los proyectos registrados.

Flags (`internal/cli/list.go:63`):

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--json` | `false` | Imprime los proyectos como JSON. |

```bash
devherd list
devherd list --json
```

Columnas en modo tabla: `NAME / FRAMEWORK / STACK / DOMAIN / STATUS / PATH`.

### 4.5 `devherd domain set <project> --domain <name>`

Define el dominio principal de un proyecto. El argumento `--domain` es obligatorio.

Flags (`internal/cli/domain.go:56`):

| Flag | Requerido | Descripcion |
|------|-----------|-------------|
| `--domain` | si | Dominio completo o nombre corto. |

Reglas de normalizacion (`internal/cli/naming.go:37`):

- Si `--domain` no contiene punto, se le agrega el TLD configurado:
  `--domain mi-demo` -> `mi-demo.test` (o `.localhost`).
- Si contiene puntos, se normaliza cada etiqueta (minusculas, guiones).
- Falla si el dominio ya pertenece a otro proyecto.

```bash
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd domain set hello-vue-flask-docker --domain demo.local.test
```

### 4.6 `devherd plan [path]`

Muestra el stack compose resuelto **sin** levantar contenedores. Si se omite la ruta, usa
el directorio actual.

```bash
devherd plan
devherd plan /ruta/al/proyecto
```

Imprime: raiz del proyecto, nombre compose (estable por ruta), origen
(manifest vs autodetect), env file, archivos compose y comandos docker de ejemplo.

### 4.7 `devherd inspect [path]`

Inspecciona un proyecto compose en busca de colisiones de infraestructura local sin
efectos secundarios. Usa la config si DevHerd esta inicializado; si no, usa defaults.

```bash
devherd inspect
devherd inspect /ruta/al/proyecto
```

Salida (`SEVERIDAD  NOMBRE  MENSAJE`), severidades `OK`/`WARN`/`FAIL`. Detecta puertos en
uso, `container_name` que colisiona con otro proyecto, volumenes externos, problemas de
`.env` (Laravel/Redis), y estado del proxy externo.

### 4.8 `devherd up [path]`

Levanta un proyecto basado en compose (`docker compose up --build -d`) desde la ruta dada
o el directorio actual. Ejecuta preflight automaticamente antes de arrancar.

Flags (`internal/cli/up.go:56-57`):

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--force` | `false` | Continua aunque el preflight detecte fallos. |
| `--no-inspect` | `false` | Omite el preflight previo. |

Comportamiento del preflight (`up.go:62`):

- Si hay **fallos** y no usas `--force`, aborta con el reporte.
- Si hay **warnings**, los muestra y continua.
- Con `--force`, continua pese a fallos.

```bash
devherd up
devherd up /ruta/al/proyecto
devherd up /ruta/al/proyecto --force
devherd up /ruta/al/proyecto --no-inspect
```

Si DevHerd no esta inicializado, `up` cae a un modo "fallback" que solo ejecuta
`docker compose` sin preflight (`up.go:25-33`).

### 4.9 `devherd stop [path]`

Detiene los contenedores del proyecto **sin** remover el estado del proxy
(`docker compose stop`).

```bash
devherd stop
devherd stop /ruta/al/proyecto
```

### 4.10 `devherd down [path]`

Detiene y elimina los contenedores del proyecto (`docker compose down`). En modo proxy
externo, ademas remueve el override compose administrado, desconecta de la red externa y
borra el bloque de dominio del Caddyfile del local_proxy
(`internal/cli/down.go:45-69`).

```bash
devherd down
devherd down /ruta/al/proyecto
```

### 4.11 `devherd open <project>`

Resuelve el dominio del proyecto y lo abre en el navegador (en Linux usa `xdg-open`). Si
no hay navegador disponible, imprime la URL.

```bash
devherd open hello-vue-flask-docker
```

La URL usa el puerto HTTP configurado (`http://dominio` si es el 80, o `http://dominio:PUERTO`).

### 4.12 `devherd proxy`

Gestiona la configuracion del proxy reverso. Subcomandos: `apply`, `bootstrap`.

#### `devherd proxy apply [project]`

Renderiza la configuracion del proxy, sincroniza `/etc/hosts` y recarga Caddy. Si se pasa
un nombre de proyecto, aplica solo ese; si no, aplica todos los registrados.

Comportamiento segun driver (`internal/cli/proxy.go:29`):

- `caddy` (host): renderiza el Caddyfile, sincroniza `/etc/hosts` (pide `sudo`) y
  valida/recarga Caddy con `sudo`.
- `caddy-docker-external`: construye el override compose, conecta los servicios a la red
  externa, fusiona los bloques de sitio en el Caddyfile del local_proxy y recarga Caddy
  dentro del contenedor.

```bash
devherd proxy apply
devherd proxy apply hello-vue-flask-docker
```

Salida tipica:

```
caddyfile: /home/usuario/.local/share/devherd/local_proxy/Caddyfile
domains: hello-vue-flask-docker.localhost
proxy status: applied
```

#### `devherd proxy bootstrap`

Crea o refresca los assets administrados del proxy externo (`docker-compose.yml`,
`Caddyfile`, `.env`, `.env.example` bajo el directorio del local_proxy). Requiere driver
`caddy-docker-external`.

Flags (`internal/cli/proxy.go:172`):

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--force` | `false` | Reescribe las plantillas compose/Caddyfile para igualar la config actual. |

```bash
devherd proxy bootstrap
devherd proxy bootstrap --force
```

### 4.13 `devherd service <start|stop|status> [service]`

Administra los servicios compartidos de desarrollo. Servicios soportados: `redis`,
`mailpit` (`internal/services/manager.go:21`).

- `service start <service>`: arranca el servicio (`docker compose up -d`), creando la red
  `infra_net` si falta. Argumento obligatorio.
- `service stop <service>`: detiene el servicio. Argumento obligatorio.
- `service status [service]`: muestra el estado (`docker compose ps`). El argumento es
  opcional; sin el, muestra todos.

```bash
devherd service start redis
devherd service start mailpit
devherd service status
devherd service status redis
devherd service stop redis
```

Puertos publicados (en `127.0.0.1`): Redis `6379`, Mailpit `1025` (SMTP) y `8025` (UI web).

### 4.14 `devherd observe ...`

Collector local de observabilidad: recibe errores estilo Sentry, los agrupa en *issues* y
ofrece un panel web. Usa una base SQLite separada de la principal.

Subcomandos (`internal/cli/observe.go:31-45`):

#### `observe start`

Arranca el collector HTTP (proceso de larga duracion).

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | `observe.DefaultAddr` | Direccion de escucha del collector. |

```bash
devherd observe start
devherd observe start --addr 127.0.0.1:9999
```

#### `observe status`

Consulta `/health` del collector.

```bash
devherd observe status
```

#### `observe open`

Abre el panel web (`/observe`) en el navegador.

```bash
devherd observe open
```

#### `observe dsn <project>`

Imprime el DSN local del proyecto (formato `http://devherd@<addr>/<project>`).

```bash
devherd observe dsn mi-app
```

#### `observe attach <project-or-path> --stack <stack>`

Genera (o previsualiza) un override compose local que inyecta el DSN local y la config de
Sentry en los servicios del proyecto.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--stack` (obligatorio) | — | `laravel`, `node`, `python`, `go`, `docker` o `generic`. |
| `--service` | todos | Servicio(s) compose a observar; repetible o separado por comas. |
| `--environment` | `local` | Valor de entorno Sentry inyectado. |
| `--addr` | `observe.DefaultAddr` | Direccion usada para construir el DSN por defecto. |
| `--dsn` | — | Sobrescribe el DSN local generado. |
| `--dry-run` | `false` | Previsualiza el override sin escribir archivos. |

```bash
devherd observe attach mi-app --stack laravel
devherd observe attach mi-app --stack node --service backend --dry-run
```

El override generado se incluye automaticamente en `up`/`stop`/`down`.

#### `observe detach <project-or-path>`

Elimina el override de observe del proyecto.

```bash
devherd observe detach mi-app
```

#### `observe scan [project]` / `observe containers [project]`

`scan` toma una instantanea de los contenedores Docker observados y la guarda;
`containers` los lista (`--limit`, default 50).

```bash
devherd observe scan
devherd observe containers --limit 20
```

#### `observe issues [project]` / `observe events [project]` / `observe timeline <event-id>`

Listan issues agrupados, eventos recientes y la linea de tiempo de fallos de un evento.
`issues` y `events` aceptan `--limit` (default 20).

```bash
devherd observe issues
devherd observe events mi-app --limit 50
devherd observe timeline <event-id>
```

#### `observe alert <add|list|remove|deliveries>`

Reglas de alerta locales. Tipos soportados (`--on`): `new-issue`, `error-rate`,
`container-exit`, `container-restart`.

```bash
devherd observe alert add --on new-issue --project mi-app
devherd observe alert add --on error-rate --threshold 5 --window 5m
devherd observe alert list
devherd observe alert remove 1
devherd observe alert deliveries --limit 20
```

#### `observe cleanup`

Elimina datos antiguos de Observe.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--days` | `14` | Elimina datos mas viejos que N dias. |

```bash
devherd observe cleanup --days 7
```

### 4.15 `devherd sentry ...`

Integracion con Sentry (estado MVP). Subcomandos: `init` (visible), `set-dsn` y `test`
(ocultos). Estos dos ultimos estan marcados `Hidden` (`internal/cli/sentry.go`) porque aun
no estan implementados: no aparecen en `devherd sentry --help`, pero siguen invocables y
devuelven `not implemented`.

#### `sentry init <project> --stack <stack>`

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--stack` (obligatorio) | — | `laravel`, `node`, `python` o `go`. |
| `--dry-run` | `false` | Previsualiza los pasos planificados sin modificar archivos. |

> Hoy solo el modo `--dry-run` esta implementado; el modo "apply" devuelve
> `not implemented` (`internal/cli/sentry.go:52`).

```bash
devherd sentry init mi-app --stack laravel --dry-run
```

#### `sentry set-dsn <project> --dsn <dsn>` / `sentry test <project>`

Ambos estan **ocultos** (`Hidden: true`) y son stubs que devuelven `not implemented` en el
MVP (`internal/cli/sentry.go`). No se listan en la ayuda hasta que se implementen.

### 4.16 `devherd logs [path]`

Transmite los logs de los contenedores del proyecto (`docker compose logs`) desde la ruta
dada o el directorio actual. A diferencia de otros comandos, la salida se conecta
directamente (sin buffer) para soportar el modo `--follow` en vivo.

Si DevHerd esta inicializado, `logs` alinea los archivos compose con los que se usaron en
`up` (override de proxy externo + observe), de modo que cubre todos los servicios en
ejecucion (`internal/cli/logs.go:36-46`). Sin inicializacion, usa el proyecto base.

Flags (`internal/cli/logs.go:57-58`):

| Flag | Default | Descripcion |
|------|---------|-------------|
| `-f`, `--follow` | `false` | Sigue la salida en vivo (streaming). |
| `--tail` | (todo) | Numero de lineas a mostrar desde el final (p. ej. `100` o `all`). |

```bash
devherd logs
devherd logs /ruta/al/proyecto
devherd logs -f
devherd logs --tail 100
devherd logs /ruta/al/proyecto -f --tail 50
```

## 5. El manifiesto `.devherd.yml`

Si un proyecto contiene `.devherd.yml`, DevHerd lo usa en lugar de autodetectar el archivo
compose (`internal/compose/project.go:194`). Formato:

```yaml
version: 1
compose:
  files:
    - docker-compose.yml
    - docker-compose.override.yml   # opcional, rutas relativas a la raiz
  env_file: .env                    # opcional
proxy:
  domain: mi-app.localhost          # dominio para el proxy
  service: web                      # servicio que recibe el trafico
  port: 8080                        # puerto interno del servicio
```

La metadata `proxy` permite que `proxy apply` enrute cualquier framework hacia el servicio
correcto, sin depender de las reglas predefinidas (`vue+flask`, `flask`, `vue`).

## 6. Flujos de trabajo

### 6.1 Modo proxy en Docker externo (recomendado, sin sudo para Caddy)

```bash
devherd init --proxy caddy-docker-external
devherd doctor

devherd park /home/usuario/develop/examples
devherd plan /home/usuario/develop/examples/hello-vue-flask-docker
devherd inspect /home/usuario/develop/examples/hello-vue-flask-docker

devherd up /home/usuario/develop/examples/hello-vue-flask-docker
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker

devherd list
```

### 6.2 Modo proxy en host (Caddy + /etc/hosts)

```bash
./scripts/install-caddy-ubuntu.sh
devherd init --proxy caddy
devherd doctor

devherd park /home/usuario/develop/examples
devherd up /home/usuario/develop/examples/mi-app
devherd domain set mi-app --domain mi-demo
devherd proxy apply mi-app   # pedira sudo para /etc/hosts y caddy reload
devherd open mi-app
```

### 6.3 Servicios compartidos + observabilidad

```bash
devherd service start redis
devherd service start mailpit
# Mailpit UI: http://127.0.0.1:8025

devherd observe start &           # collector local
devherd observe attach mi-app --stack node
devherd up /ruta/a/mi-app
devherd observe open
devherd observe issues mi-app
```

### 6.4 Bajar todo

```bash
devherd down /ruta/a/mi-app     # detiene contenedores y limpia el proxy
devherd service stop redis
devherd service stop mailpit
```

## 7. Donde vive el estado

| Que | Ruta (Linux) |
|-----|--------------|
| Config | `~/.config/devherd/config.json` |
| Base de datos | `~/.local/share/devherd/devherd.db` |
| Proxy en host (Caddyfile) | `~/.local/share/devherd/proxy/Caddyfile` |
| Proxy externo (local_proxy) | `~/.local/share/devherd/local_proxy/` |
| Servicios compartidos | `~/.local/share/devherd/compose/shared-services/` |
| Logs / estado | `~/.local/state/devherd/` |
| Override de proxy (por proyecto) | `<proyecto>/.devherd.proxy.override.yml` |

Estas rutas se derivan de `internal/config/paths.go`.
</content>
