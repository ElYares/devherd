# DevHerd CLI: Documentacion de Comandos

Esta guia describe la CLI actual de DevHerd, su alcance en el MVP y el estado real de cada comando.

## Estado de la CLI

### Funciona hoy

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list`
- `devherd list --json`
- `devherd domain set <project> --domain <name>`
- `devherd proxy apply [project]`
- `devherd plan [path]`
- `devherd inspect [path]`
- `devherd up [path]`
- `devherd down [path]`
- `devherd open <project>`
- `devherd sentry init <project> --stack <stack> --dry-run`

### Existe pero aun no esta implementado

- `devherd logs`
- `devherd service start|stop|status`
- `devherd sentry set-dsn`
- `devherd sentry test`

## Uso general

```bash
devherd [command] [flags]
```

Ayuda general:

```bash
devherd --help
```

Version:

```bash
devherd --version
```

## Desde donde correr los comandos

Si usas:

```bash
go run ./cmd/devherd ...
```

debes estar en la raiz del repositorio, porque `go run ./cmd/devherd` depende de esa ruta relativa.

Si instalas el binario:

```bash
./scripts/install-ubuntu.sh
```

entonces ya puedes ejecutar:

```bash
devherd ...
```

desde cualquier carpeta del sistema.

Regla practica:

- Comandos de desarrollo del repo: desde la raiz.
- Binario instalado: desde cualquier carpeta.
- Comandos que aceptan ruta como `park`, `up` y `down`: mejor pasar la ruta explicita si no estas dentro del proyecto.

## Flujo recomendado de prueba hoy

Para no escribir configuracion real en tu home mientras pruebas:

```bash
export XDG_CONFIG_HOME=/tmp/devherd-config
export XDG_DATA_HOME=/tmp/devherd-data
export XDG_STATE_HOME=/tmp/devherd-state
export GOCACHE=/tmp/devherd-gocache
```

Flujo minimo:

```bash
go run ./cmd/devherd init --proxy caddy-docker-external
go run ./cmd/devherd doctor
go run ./cmd/devherd park /home/elyarestark/develop/examples
go run ./cmd/devherd plan /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd inspect /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd domain set hello-vue-flask-docker --domain mi-demo
go run ./cmd/devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd proxy apply hello-vue-flask-docker
go run ./cmd/devherd open hello-vue-flask-docker
```

## Comandos

### `devherd init`

Inicializa directorios locales de DevHerd, crea el archivo de configuracion y prepara la base SQLite.

Sintaxis:

```bash
devherd init [flags]
```

Flags:

- `--proxy string`: driver de proxy. Valores soportados hoy: `caddy`, `nginx`, `caddy-docker-external`.
- `--tld string`: TLD local. Default: `test`.
- `--runtime-manager string`: gestor de runtimes. Valores soportados hoy: `mise`, `asdf`.

Ejemplos:

```bash
devherd init
devherd init --proxy caddy --tld test --runtime-manager mise
devherd init --proxy caddy-docker-external
```

Salida esperada:

- Ruta del archivo de configuracion.
- Ruta de la base SQLite.
- Driver de proxy configurado.
- TLD local.
- Runtime manager configurado.
- Estado de creacion o reutilizacion de config y base.

Notas:

- Es idempotente.
- Si la config ya existe, la reutiliza y actualiza segun flags.
- Si eliges `--proxy caddy-docker-external` y no pasas `--tld`, DevHerd cambia el TLD default a `localhost`.

### `devherd doctor`

Valida prerequisitos del host para el MVP.

Sintaxis:

```bash
devherd doctor
```

Chequeos actuales:

- Escritura en rutas XDG locales.
- Binario `docker`.
- Acceso al daemon Docker.
- `docker compose`.
- Segun el driver configurado:
  - modo `caddy`: binario `caddy`, `dnsmasq` y puertos TCP `80` y `443`
  - modo `caddy-docker-external`: `/home/elyarestark/infra/local_proxy/docker-compose.yml`, `/home/elyarestark/infra/local_proxy/Caddyfile` y puerto TCP `80`

Comportamiento:

- Imprime una linea por chequeo.
- Devuelve exit code distinto de cero si hay fallos.
- Usa `WARN` para condiciones no bloqueantes como puertos ocupados.
- En el modo `caddy`, `dnsmasq` es opcional porque `proxy apply` sincroniza un bloque en `/etc/hosts`.

Ejemplo:

```bash
devherd doctor
```

Salida tipo:

```text
OK    local paths      writable XDG directories ready
OK    Docker CLI       found at /usr/bin/docker
FAIL  Caddy            caddy not found in PATH
```

### `devherd park`

Registra un directorio para descubrimiento automatico de proyectos.

Sintaxis:

```bash
devherd park <path>
```

Ejemplos:

```bash
devherd park ~/Code
devherd park /home/elyarestark/develop/examples
```

Comportamiento actual:

- Guardar el path en SQLite.
- Recorrer el directorio y sus hijos inmediatos.
- Detectar proyectos Laravel, Node.js, Go, Python y Docker.
- Registrar un proyecto por carpeta detectada.
- Generar un dominio principal `<proyecto>.test`.

Deteccion actual:

- Laravel via `artisan` + `composer.json`
- Node/Vue via `package.json`
- Python/Flask via `requirements.txt`, `pyproject.toml` o `app.py`
- Go via `go.mod`
- Docker via `docker-compose.yml`, `compose.yml` o `Dockerfile`

Ejemplo de salida:

```text
parked: /home/elyarestark/develop/examples
detected projects: 1
```

### `devherd list`

Lista proyectos registrados.

Sintaxis:

```bash
devherd list
devherd list --json
```

Comportamiento actual:

- Mostrar nombre del proyecto.
- Ruta local.
- Stack detectado.
- Dominio principal.
- Estado general.
- Soporta salida tabular y JSON.

Ejemplo:

```bash
devherd list
devherd list --json
```

Ejemplo de salida:

```text
NAME                    FRAMEWORK  STACK               DOMAIN                       STATUS    PATH
hello-vue-flask-docker  vue+flask  node+python+docker  hello-vue-flask-docker.test  detected  /ruta/al/proyecto
```

### `devherd domain`

Permite personalizar el dominio principal de un proyecto.

#### `devherd domain set`

Sintaxis:

```bash
devherd domain set <project> --domain <nombre>
```

Comportamiento actual:

- Cambia el dominio principal guardado en SQLite.
- Si pasas un nombre corto como `mi-demo`, DevHerd lo convierte a `mi-demo.test`.
- Si pasas un dominio completo como `mi-demo.local`, DevHerd lo conserva normalizado.
- El dominio personalizado sobrevive a futuros `devherd park`.

Ejemplos:

```bash
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd domain set hello-vue-flask-docker --domain mi-demo.test
devherd domain set hello-vue-flask-docker --domain api-lab.local
```

Ejemplo de salida:

```text
project: hello-vue-flask-docker
primary domain: mi-demo.test
```

### `devherd proxy`

Administra la configuracion del reverse proxy local.

#### `devherd proxy apply`

Sintaxis:

```bash
devherd proxy apply [project]
```

Comportamiento actual:

- Soporta dos modos segun el driver configurado.

Modo `caddy`:

- Renderiza un `Caddyfile` administrado por DevHerd.
- Sincroniza dominios registrados en un bloque administrado de `/etc/hosts`.
- Valida la configuracion de Caddy.
- Si Caddy ya esta corriendo, intenta `reload`.
- Si no esta corriendo, intenta `start`.
- Pide privilegios `sudo` para actualizar `/etc/hosts` y levantar o recargar Caddy.

Modo `caddy-docker-external`:

- Reutiliza `/home/elyarestark/infra/local_proxy`.
- Lee `proxy.domain`, `proxy.service` y `proxy.port` desde `.devherd.yml` cuando existen.
- Genera `.devherd.proxy.override.yml` dentro del proyecto.
- Conecta los servicios necesarios a la red externa `infra_web`.
- Escribe o reemplaza el bloque del dominio administrado en `/home/elyarestark/infra/local_proxy/Caddyfile`.
- Levanta `local_proxy` con `docker compose up -d` si hace falta.
- Valida y recarga Caddy dentro del contenedor `infra_caddy`.

Alcance actual del routing:

- `vue+flask`
- `/api/*` -> `127.0.0.1:8000`
- `/` -> `127.0.0.1:5173`
- `flask`
- `/` -> `127.0.0.1:8000`
- `vue`
- `/` -> `127.0.0.1:5173`

En modo `caddy-docker-external`, si el manifiesto trae:

```yaml
proxy:
  domain: aang.localhost
  service: web
  port: 80
```

DevHerd enruta `http://aang.localhost` al alias administrado para el servicio `web`.

Ejemplos:

```bash
devherd proxy apply
devherd proxy apply hello-vue-flask-docker
```

Notas:

- Si tu proyecto esta servido por Docker, conviene levantarlo antes con `devherd up`.
- En modo `caddy`, requiere `caddy` instalado en host.
- En modo `caddy-docker-external`, requiere que exista `/home/elyarestark/infra/local_proxy`.
- Si pasas un nombre de proyecto como `aang-server`, ese proyecto debe estar registrado en SQLite via `devherd park`.

### `devherd plan`

Inspecciona el stack Compose resuelto sin iniciar contenedores.

Sintaxis:

```bash
devherd plan [path]
```

Comportamiento actual:

- Si no pasas `path`, usa el directorio actual.
- Si existe `.devherd.yml`, usa `compose.files` y `compose.env_file` desde ese manifiesto.
- Si no existe manifiesto, autodetecta un unico archivo Compose soportado.
- Calcula un project-name estable por ruta, con forma `devherd-<slug>-<hash>`.
- Imprime:
  - raiz del proyecto
  - project-name de Compose
  - fuente de resolucion
  - archivo `.env` detectado
  - archivos Compose incluidos
  - comando base de `docker compose`
  - ejemplos de `config`, `up` y `down`
- No ejecuta Docker ni modifica archivos.

Ejemplos:

```bash
devherd plan
devherd plan /home/elyarestark/develop-work/aang-server
```

### `devherd inspect`

Audita un proyecto Compose para detectar colisiones locales y riesgos de aislamiento.

Sintaxis:

```bash
devherd inspect [path]
```

Comportamiento actual:

- Si no pasas `path`, usa el directorio actual.
- Resuelve el mismo stack que `devherd plan`.
- Lee el `.env` efectivo del proyecto.
- Consulta Docker cuando esta disponible.
- Revisa puertos publicados y quien los ocupa.
- Revisa `container_name` fijos o parametrizados.
- Revisa si el dominio del proxy externo esta publicado y si el servicio objetivo esta corriendo.
- Revisa señales comunes de mezcla local en Laravel: `APP_URL`, `SESSION_COOKIE`, `CACHE_PREFIX`, `REDIS_PREFIX`, `REDIS_DB` y `REDIS_CACHE_DB`.
- Revisa volumenes externos declarados por Compose.

Ejemplos:

```bash
devherd inspect
devherd inspect /home/elyares/develop/work/aang-server
devherd inspect /home/elyares/develop/work/Uniformes
```

Salida tipo:

```text
Project root: /home/elyares/develop/work/aang-server
Findings:
WARN  shared-service   project can reach shared Redis on infra_net; namespace Redis keys per project
OK    container_name   app uses parameterized name "aang_app"
OK    port             web publishes 127.0.0.1:8083 and it is already owned by this project
OK    proxy            aang.localhost is published and web is running
```

### Patron recomendado para proyectos Compose

Para evitar choques al levantar clones o variantes paralelas, los proyectos deben parametrizar `container_name` con `COMPOSE_NAME_PREFIX` y mantener defaults compatibles:

```yaml
services:
  app:
    container_name: ${COMPOSE_NAME_PREFIX:-aang}_app
  web:
    container_name: ${COMPOSE_NAME_PREFIX:-aang}_web
```

En `.env`:

```env
COMPOSE_NAME_PREFIX=aang
APP_URL=http://aang.localhost
SESSION_COOKIE=aang_session
CACHE_PREFIX=aang_cache_
REDIS_PREFIX=aang_database_
REDIS_DB=7
REDIS_CACHE_DB=8
APP_PORT=8083
FORWARD_DB_PORT=3310
```

Para un clone paralelo, cambia el prefijo, dominio, puertos, cookie y prefijos de cache/Redis:

```env
COMPOSE_NAME_PREFIX=aang-v2
APP_URL=http://aang-v2.localhost
SESSION_COOKIE=aang_v2_session
CACHE_PREFIX=aang_v2_cache_
REDIS_PREFIX=aang_v2_database_
APP_PORT=8084
FORWARD_DB_PORT=3311
```

### `devherd up`

Levanta un proyecto basado en Docker Compose.

Sintaxis:

```bash
devherd up [path]
```

Flags:

- `--force`: continua aunque el preflight detecte fallos.
- `--no-inspect`: omite el preflight antes de levantar el proyecto.

Comportamiento actual:

- Si no pasas `path`, usa el directorio actual.
- Busca `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` o `compose.yaml`.
- Si existe `.devherd.yml`, usa `compose.files` y `compose.env_file` desde ese manifiesto.
- Ejecuta un preflight equivalente a `devherd inspect` antes de levantar.
- Si el preflight detecta `FAIL`, aborta antes de tocar Docker.
- Si pasas `--force`, imprime los fallos y continua.
- Si pasas `--no-inspect`, levanta sin auditar.
- Ejecuta Compose con `--project-name devherd-<slug>-<hash>` para que clones con el mismo basename no compartan project-name.
- Si el driver es `caddy-docker-external`, puede agregar `.devherd.proxy.override.yml` al levantar el proyecto.
- Ejecuta `docker compose up --build -d`.

Salida de preflight con warnings:

```text
preflight: warnings found
Project root: /home/elyares/develop/work/aang-server
Findings:
WARN  shared-service   project can reach shared Redis on infra_net; namespace Redis keys per project

continuing...
```

Salida de preflight con fallos:

```text
preflight: failures found
Project root: /home/elyares/develop/work/demo
Findings:
FAIL  port             web wants 8082 but other_container owns it
```

Manifiesto opcional:

```yaml
version: 1
compose:
  files:
    - docker-compose.yml
    - docker-compose.shared.yml
  env_file: .env.devherd
```

Notas del manifiesto:

- `compose.files` debe usar rutas relativas al proyecto.
- `compose.env_file` es opcional.
- Si el manifiesto existe y es valido, reemplaza la autodeteccion simple de un solo archivo Compose.

Notas de volumenes:

- Cambiar el project-name de Compose tambien cambia el nombre default de los volumenes internos.
- Para preservar datos entre cambios de project-name o clones, define `name:` en el volumen y parametrizalo desde `.env`.
- Ejemplo:

```yaml
volumes:
  db_data:
    name: ${DB_VOLUME_NAME:-mi_proyecto_db_data}
    external: ${DB_VOLUME_EXTERNAL:-false}
```

```env
DB_VOLUME_NAME=mi_proyecto_db_data
DB_VOLUME_EXTERNAL=false
```

Ejemplos:

```bash
devherd up
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd up --force /home/elyares/develop/work/aang-server
devherd up --no-inspect /home/elyares/develop/work/aang-server
```

### `devherd down`

Detiene un proyecto basado en Docker Compose.

Sintaxis:

```bash
devherd down [path]
```

Comportamiento actual:

- Si no pasas `path`, usa el directorio actual.
- Busca un archivo Compose soportado.
- Si existe `.devherd.yml`, usa los mismos `compose.files` y `compose.env_file` definidos ahi.
- Si existe `.devherd.proxy.override.yml`, lo reutiliza para bajar el mismo stack que se levanto en modo externo.
- Ejecuta `down` con el project-name estable y tambien intenta limpiar el project-name legado derivado de la carpeta.
- En modo `caddy-docker-external`, tambien elimina el override generado y remueve el bloque del dominio del `Caddyfile` externo.
- Ejecuta `docker compose down`.

Ejemplos:

```bash
devherd down
devherd down /home/elyarestark/develop/examples/hello-vue-flask-docker
```

### `devherd open`

Abre un proyecto en el navegador.

Sintaxis:

```bash
devherd open <project>
```

Ejemplos:

```bash
devherd open hello-vue-flask-docker
```

Comportamiento actual:

- Lee el dominio principal guardado en SQLite.
- Construye la URL HTTP usando la configuracion actual del proxy.
- Intenta abrirla con `xdg-open`.
- Si `xdg-open` no existe, imprime la URL.

### `devherd logs`

Mostrara o seguira logs de un proyecto.

Sintaxis objetivo:

```bash
devherd logs <project>
```

Comportamiento esperado:

- Unificar logs de app, proxy y contenedores relacionados.
- En proyectos compuestos, mostrar frontend y backend.

Estado actual:

- Comando disponible.
- Aun devuelve `not implemented yet`.

### `devherd service`

Administrara servicios compartidos del entorno local.

Subcomandos:

```bash
devherd service start <service>
devherd service stop <service>
devherd service status [service]
```

Alcance del MVP:

- `redis`
- `mailpit`

Ejemplos objetivo:

```bash
devherd service start redis
devherd service start mailpit
devherd service status redis
```

Comportamiento esperado:

- Levantar contenedores Compose administrados por DevHerd.
- Crear la red Docker compartida `infra_net` cuando haga falta.
- Conectar servicios compartidos a `infra_net` con aliases estables (`redis`, `mailpit`).
- Exponer puertos conocidos.

Estado actual:

- `redis` levanta `infra_redis` con `redis:7-alpine`, volumen persistente y puerto host `127.0.0.1:6379`.
- `mailpit` levanta `infra_mailpit` con puertos host `127.0.0.1:1025` y `127.0.0.1:8025`.
- `status` consulta el stack Compose administrado por DevHerd.

### `devherd sentry`

Administra integracion de Sentry por proyecto.

#### `devherd sentry init`

Inicializa el flujo de bootstrap de Sentry para un proyecto.

Sintaxis:

```bash
devherd sentry init <project> --stack <stack> [--dry-run]
```

Flags:

- `--stack string`: requerido. Valores planeados: `laravel`, `node`, `python`, `go`.
- `--dry-run`: muestra el plan sin tocar archivos.

Ejemplos:

```bash
devherd sentry init hello-vue-flask-docker --stack python --dry-run
devherd sentry init hello-vue-flask-docker --stack node --dry-run
```

Comportamiento actual:

- Si usas `--dry-run`, imprime el plan de cambios.
- Sin `--dry-run`, el modo apply aun no esta implementado.

Uso recomendado hoy:

- Backend Flask del ejemplo: `--stack python`
- Frontend Vue del ejemplo: `--stack node`

#### `devherd sentry set-dsn`

Guardara un DSN por proyecto.

Sintaxis objetivo:

```bash
devherd sentry set-dsn <project> --dsn <dsn>
```

Estado actual:

- Comando disponible.
- Aun devuelve `not implemented yet`.

#### `devherd sentry test`

Enviara un evento de prueba a Sentry.

Sintaxis objetivo:

```bash
devherd sentry test <project>
```

Estado actual:

- Comando disponible.
- Aun devuelve `not implemented yet`.

## Ejemplo con el proyecto Vue + Flask + Docker

Proyecto de prueba movido fuera del repo:

- `/home/elyarestark/develop/examples/hello-vue-flask-docker`

Flujo actual:

```bash
go run ./cmd/devherd init
go run ./cmd/devherd doctor
go run ./cmd/devherd park /home/elyarestark/develop/examples
go run ./cmd/devherd domain set hello-vue-flask-docker --domain mi-demo.test
go run ./cmd/devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd proxy apply hello-vue-flask-docker
go run ./cmd/devherd open hello-vue-flask-docker
go run ./cmd/devherd list
go run ./cmd/devherd sentry init hello-vue-flask-docker --stack python --dry-run
go run ./cmd/devherd sentry init hello-vue-flask-docker --stack node --dry-run
go run ./cmd/devherd down /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Flujo actual si instalas el binario:

```bash
devherd init
devherd doctor
devherd park /home/elyarestark/develop/examples
devherd domain set hello-vue-flask-docker --domain mi-demo.test
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker
devherd list
```

## Comandos futuros probables

Estos no existen aun, pero tienen sentido por el tipo de proyectos compuestos que estamos usando:

- `devherd up <project-name>`
- `devherd down <project-name>`
- `devherd proxy reload`
- `devherd domains refresh`
- `devherd doctor --json`
