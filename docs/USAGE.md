# Guia de uso de DevHerd

Referencia **canonica** de comandos y flags de DevHerd, derivada del codigo real.
Instalacion, todos los comandos, sus efectos sobre tu sistema, ejemplos y flujos de
trabajo comunes.

> DevHerd esta en estado MVP/alpha y es **Ubuntu/Linux-first**. Requiere Docker (con
> `docker compose`) y, para el modo de proxy en host, el binario `caddy`.

Documentos relacionados:

- [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md) — analisis del sistema, estado real y deuda.
- [ARCHITECTURE.md](ARCHITECTURE.md) — paquetes internos, tipos y flujo de datos.
- [project-workflow.md](project-workflow.md) — flujos narrativos end-to-end.
- [guides/scaffold.md](guides/scaffold.md) — detalle del generador de compose.

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
  (`infra_caddy`, cuyo compose y Caddyfile viven en `~/.local/share/devherd/local_proxy/`)
  y las redes Docker `infra_web` e `infra_net`.

> El valor `nginx` se acepta en `--proxy` y se persiste, pero **no existe driver nginx**:
> `proxy apply` cae al renderer de Caddy en host.

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

> Este script **no inyecta los metadatos de version**, asi que el binario instalado por
> esta via reporta siempre `0.1.0-alpha (commit dev, built unknown)`. Para tener version
> real, usa `make build` o `make install`.

### 2.3 Build manual

El binario no se versiona en el repositorio (`bin/` y `/devherd` estan en `.gitignore`).
La via recomendada es el `Makefile`, que inyecta los metadatos de version
(`version.Version/Commit/Date`) via `-ldflags`:

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
  `Version (commit X, built Y)`.
- `--verbose`: habilita logging de diagnostico a nivel DEBUG en **stderr**.
- `--log-json`: emite los logs de diagnostico como JSON en **stderr** (util para scripting
  o agregadores de logs).

`--verbose` y `--log-json` son persistentes y aplican a cualquier subcomando. El logging
de diagnostico se separa de la salida "de producto": los mensajes utiles al usuario van a
**stdout** y los diagnosticos a **stderr**, de modo que puedes redirigir cada flujo por
separado. Sin `--verbose` el nivel por defecto es INFO.

```bash
devherd --verbose up /ruta/al/proyecto
devherd --verbose --log-json observe start 2> devherd.log
devherd list --json > proyectos.json 2> diagnostico.log
```

Casi todos los comandos que operan sobre proyectos requieren haber ejecutado
`devherd init` antes; de lo contrario fallan con *"DevHerd is not initialized. Run
`devherd init` first"*.

> **Efecto lateral a tener en cuenta:** cargar el contexto de aplicacion **crea los
> directorios locales y la base SQLite** aunque el comando parezca de solo lectura. Esto
> aplica a `doctor`, `inspect` y `logs`. `service` tambien crea los directorios (llama a
> `paths.Ensure()` en cada invocacion) pero no la base SQLite. Los unicos comandos que no
> crean nada son `init` y `plan`.

## 4. Referencia de comandos

Los 19 comandos de primer nivel, en el mismo orden en que los registra la CLI.

### 4.1 `devherd init`

Inicializa los directorios locales, la config (`config.json`) y la base SQLite. Si el
driver es `caddy-docker-external`, ademas crea los assets del proxy externo.

| Flag | Default | Valores | Descripcion |
|------|---------|---------|-------------|
| `--proxy` | `caddy` | `caddy`, `nginx`, `caddy-docker-external` | Driver de proxy reverso. |
| `--tld` | `test` | cualquier TLD | Dominio de nivel superior local. |
| `--runtime-manager` | `mise` | `mise`, `asdf` | Gestor de runtimes. |

Notas:

- **Los flags solo se aplican si los cambias explicitamente.** Re-ejecutar `devherd init`
  sobre una config existente **no resetea** nada a los defaults.
- Si pasas `--proxy caddy-docker-external` sin `--tld`, el TLD por defecto pasa a
  `localhost`.
- `init` es seguro de re-ejecutar: reusa la config existente y reporta el estado
  (`created`/`reused`, `migrated`).
- Cambiar de driver **no reescribe los dominios ya guardados** con el TLD anterior.

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
distinto de 0 si hay **fallos** (los warnings no afectan al codigo de salida).

Funciona **sin** `devherd init`: si no encuentra config, usa los valores por defecto.

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

Checks comunes: rutas locales, Docker CLI, daemon Docker, modo del engine (exige Linux) y
`docker compose`. En modo `caddy-docker-external` anade directorio/compose/Caddyfile del
local_proxy, redes `infra_web` e `infra_net`, sufijo administrado y puerto 80 del
contenedor. En modo `caddy` anade el binario `caddy`, `dnsmasq` (opcional) y los puertos
80 y 443.

### 4.3 `devherd park <path>`

Registra un directorio para descubrimiento automatico de proyectos y los inserta o
actualiza en la base. La ruta es **obligatoria**.

```bash
devherd park /home/usuario/develop/examples
```

Imprime los proyectos detectados con columnas `NAME / FRAMEWORK / STACK / DOMAIN / PATH`.
El dominio principal se deriva del nombre del proyecto y el TLD configurado
(p. ej. `mi-app.test`). Los dominios personalizados previos se conservan.

> `park` tambien **elimina** de la base los proyectos detectados bajo esa ruta que ya no
> existen. Los proyectos con dominio asignado manualmente conservan su dominio, pero un
> proyecto borrado del disco desaparece del registro.

Solo escanea el directorio indicado y **un nivel** de subdirectorios.

### 4.4 `devherd list`

Lista los proyectos registrados.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--json` | `false` | Imprime los proyectos como JSON. |

```bash
devherd list
devherd list --json
```

Columnas en modo tabla: `NAME / FRAMEWORK / STACK / DOMAIN / STATUS / PATH`.

### 4.5 `devherd domain set <project> --domain <name>`

Define el dominio principal de un proyecto.

| Flag | Requerido | Descripcion |
|------|-----------|-------------|
| `--domain` | si | Dominio completo o nombre corto. |

Reglas de normalizacion:

- Si `--domain` no contiene punto, se le agrega el TLD configurado:
  `--domain mi-demo` → `mi-demo.test` (o `.localhost`).
- Si contiene puntos, se normaliza cada etiqueta (minusculas, guiones).
- Falla si el dominio ya pertenece a otro proyecto.

```bash
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd domain set hello-vue-flask-docker --domain demo.local.test
```

> Cambiar el dominio **no** re-aplica el proxy ni actualiza `/etc/hosts`. Ejecuta
> `devherd proxy apply` despues.

### 4.6 `devherd proxy`

Gestiona la configuracion del proxy reverso. Subcomandos: `apply`, `bootstrap`.

#### `devherd proxy apply [project]`

Renderiza la configuracion del proxy, sincroniza `/etc/hosts` y recarga Caddy. Si se pasa
un nombre de proyecto, aplica solo ese; si no, aplica todos los registrados.

Comportamiento segun driver:

- **`caddy-docker-external`**: construye el override compose, conecta los servicios a la
  red externa, fusiona los bloques de sitio en el Caddyfile del local_proxy y recarga
  Caddy dentro del contenedor.
- **`caddy` (host)**: renderiza el Caddyfile completo, sincroniza `/etc/hosts` (pide
  `sudo`) y valida/recarga Caddy con `sudo`.

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

Cosas importantes que hace este comando sobre tu sistema:

- **Pide `sudo` siempre** para actualizar `/etc/hosts`, incluso en modo externo y aunque
  el contenido resultante sea identico. Avisa por stderr antes de pedirlo.
- **Sincroniza los dominios de _todos_ los proyectos registrados**, no solo el que
  filtres por argumento.
- **Escribe `.devherd.proxy.override.yml` dentro del repositorio** del proyecto.
- En modo `caddy` (host) **regenera el Caddyfile entero**: aplicar un solo proyecto borra
  los sitios de los demas.
- En modo `caddy` (host) solo soporta los frameworks `vue+flask`, `flask` y `vue`, con
  puertos fijos en loopback, e **ignora la metadata `proxy` del manifiesto**.

#### `devherd proxy bootstrap`

Crea o refresca los assets administrados del proxy externo (`docker-compose.yml`,
`Caddyfile`, `.env`, `.env.example` bajo el directorio del local_proxy). Requiere driver
`caddy-docker-external`.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--force` | `false` | Reescribe compose, Caddyfile y `.env.example` para igualar la config actual. |

El archivo `.env` **nunca** se sobrescribe, ni siquiera con `--force`.

```bash
devherd proxy bootstrap
devherd proxy bootstrap --force
```

### 4.7 `devherd plan [path]`

Muestra el stack compose resuelto **sin** levantar contenedores ni tocar nada. Si se omite
la ruta, usa el directorio actual. Es el unico comando totalmente libre de efectos: no
abre la base de datos ni crea directorios.

```bash
devherd plan
devherd plan /ruta/al/proyecto
```

Imprime: raiz del proyecto, nombre compose (estable por ruta), origen
(manifest vs autodetect), env file, archivos compose y comandos docker de ejemplo.

> El `Base command` que muestra `plan` **no incluye los overrides** de proxy y observe,
> asi que puede diferir del comando que realmente ejecuta `devherd up`.

### 4.8 `devherd inspect [path]`

Inspecciona un proyecto compose en busca de colisiones de infraestructura local sin
efectos sobre los contenedores. Usa la config si DevHerd esta inicializado; si no, usa
defaults. **Nunca devuelve error**, aunque encuentre fallos: es informativo.

```bash
devherd inspect
devherd inspect /ruta/al/proyecto
```

Salida (`SEVERIDAD  NOMBRE  MENSAJE`), severidades `OK`/`WARN`/`FAIL`. Detecta:

- `container_name` que colisiona con un contenedor de otro proyecto compose.
- Puertos publicados ya ocupados (distingue si el dueno es el propio proyecto).
- Volumenes declarados como `external: true`.
- Problemas de `.env` estilo Laravel: `APP_URL` incoherente con el dominio,
  `SESSION_COOKIE` sin definir, prefijos de Redis/cache ausentes o bases logicas
  duplicadas.
- Servicios conectados a la red compartida `infra_net`.
- Estado del proxy externo: bloques publicados sin servicio arriba y viceversa.

### 4.9 `devherd scaffold [path]`

Genera un `docker-compose.devherd.yml` y un `.devherd.yml` para repositorios que aun no
estan contenerizados. Detalle completo en [guides/scaffold.md](guides/scaffold.md).

Stacks reconocidos: combo `vue+flask`, `laravel`, `vue`, `flask`, `node`, `go`.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--dry-run` | `false` | Imprime compose y manifiesto sin escribir nada. |
| `--up` | `false` | Tras escribir, ejecuta `devherd up`. |
| `--force` | `false` | Sobrescribe los archivos generados si ya existen. |
| `--db` | (pregunta) | `mysql`, `mariadb`, `postgres`, `mongodb` o `none`. |
| `--redis` | **`true`** | Incluye Redis. Para omitirlo hay que pasar `--redis=false`. |

```bash
devherd scaffold                      # pregunta la base de datos por stdin
devherd scaffold --db postgres        # sin interaccion
devherd scaffold --dry-run            # previsualiza
devherd scaffold --db none --up       # genera y levanta
```

Notas de comportamiento:

- **Es interactivo por defecto**: si no pasas `--db`, abre un menu que lee de stdin. En
  scripts y CI, pasa siempre `--db`.
- **En Laravel, `--db` y `--redis` se ignoran**: la base de datos y las credenciales se
  derivan del `.env` del propio repositorio, y Redis se anade siempre.
- Los archivos se llaman `docker-compose.devherd.yml` y `.devherd.yml` a proposito, para
  no pisar un `docker-compose.yml` tuyo.
- Los puertos de host se eligen buscando uno libre a partir del preferido del stack.

### 4.10 `devherd up [path]`

Levanta un proyecto basado en compose (`docker compose up --build -d`) desde la ruta dada
o el directorio actual. Ejecuta preflight automaticamente antes de arrancar.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--force` | `false` | Continua aunque el preflight detecte fallos. |
| `--no-inspect` | `false` | Omite el preflight previo. |

Comportamiento del preflight:

- Si hay **fallos** y no usas `--force`, aborta con el reporte.
- Si hay **warnings**, los muestra y continua.
- Con `--force`, continua pese a los fallos.

```bash
devherd up
devherd up /ruta/al/proyecto
devherd up /ruta/al/proyecto --force
devherd up /ruta/al/proyecto --no-inspect
```

Dos comportamientos que conviene conocer:

- **Puede ser interactivo**: si el proyecto no tiene ningun archivo compose pero DevHerd
  reconoce el stack, pregunta `¿Genero uno con scaffold? [Y/n]` por stdin.
- **Modo degradado**: si DevHerd no esta inicializado, `up` ejecuta `docker compose`
  directo, **sin preflight, sin override de proxy y sin override de observe**.

### 4.11 `devherd serve [path]`

Comando compuesto: encadena `up` + `proxy apply` + `open` en una sola invocacion.

```bash
devherd serve
devherd serve /ruta/al/proyecto
```

No tiene flags propios. Detalles:

- Ejecuta `proxy apply` **sin filtrar por proyecto**, asi que aplica sobre todos los
  registrados y sincroniza `/etc/hosts` completo (**pide `sudo`**).
- Si no consigue resolver el nombre del proyecto para abrirlo, lo dice y termina bien;
  puedes abrirlo despues con `devherd open <proyecto>`.
- Si el navegador falla, avisa por stderr pero **no** devuelve error.

### 4.12 `devherd stop [path]`

Detiene los contenedores del proyecto (`docker compose stop`) **sin** remover el estado
del proxy.

```bash
devherd stop
devherd stop /ruta/al/proyecto
```

Conserva el bloque de dominio en el Caddyfile, las entradas de `/etc/hosts` y el archivo
`.devherd.proxy.override.yml` en el repositorio.

### 4.13 `devherd down [path]`

Detiene y elimina los contenedores del proyecto (`docker compose down`). En modo proxy
externo, ademas borra el override compose administrado y elimina el bloque de dominio del
Caddyfile del local_proxy.

```bash
devherd down
devherd down /ruta/al/proyecto
```

Notas:

- **No limpia `/etc/hosts`**: las entradas de dominio permanecen.
- Puede **arrancar** el contenedor del proxy si estaba apagado, porque necesita recargar
  su configuracion.
- Ejecuta una segunda pasada de `down` con el nombre de proyecto antiguo (previo al
  esquema con hash) para limpiar stacks creados por versiones anteriores.

### 4.14 `devherd open <project>`

Resuelve el dominio del proyecto y lo abre en el navegador (en Linux usa `xdg-open`). Si
no hay navegador disponible, imprime la URL.

```bash
devherd open hello-vue-flask-docker
```

La URL usa el puerto HTTP configurado (`http://dominio` si es el 80, o
`http://dominio:PUERTO`). **Siempre es `http://`**: el proxy no expone HTTPS.

### 4.15 `devherd logs [path]`

Transmite los logs de los contenedores del proyecto (`docker compose logs`) desde la ruta
dada o el directorio actual. La salida se conecta directamente, sin buffer, para soportar
el modo `--follow` en vivo.

Si DevHerd esta inicializado, `logs` alinea los archivos compose con los que se usaron en
`up` (override de proxy externo + observe), de modo que cubre todos los servicios en
ejecucion.

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

> El argumento es una **ruta**, no un nombre de proyecto registrado. No hay flag para
> filtrar por servicio.

### 4.16 `devherd service <start|stop|status> [service]`

Administra los servicios compartidos de desarrollo. Servicios soportados: `redis`,
`mailpit`, `prometheus` y `grafana`.

- `service start <service>`: arranca el servicio (`docker compose up -d`), creando la red
  `infra_net` si falta. Argumento obligatorio. Con `--force` devuelve los archivos de
  configuracion administrados a la plantilla de DevHerd, guardando antes una copia `.bak`.
- `service stop <service>`: detiene el servicio. Argumento obligatorio.
- `service status [service]`: muestra el estado (`docker compose ps`). Argumento opcional.
  **No escribe nada**: si nunca se arranco un servicio, lo dice en vez de crear el stack.

```bash
devherd service start redis
devherd service start mailpit
devherd service status
devherd service status redis
devherd service stop redis
```

Puertos publicados (en `127.0.0.1`): Redis `6379`, Mailpit `1025` (SMTP) y `8025` (UI web),
Prometheus `9090`.

#### Prometheus

`devherd service start prometheus` lo levanta **ya apuntado al collector de Observe**,
que publica sus metricas en `/metrics` (ver `docs/observe.md`). No hay que escribir el
`prometheus.yml`: DevHerd lo genera con la direccion correcta.

Esa direccion **no es `127.0.0.1`**. Prometheus corre dentro de un contenedor, y desde
ahi loopback es el propio contenedor: lo que se escribe es el gateway de `infra_net`,
que es la red donde vive. Antes de arrancar, DevHerd **sonda el collector desde un
contenedor** y avisa si no responde, en vez de dejar un target caido que se descubre
media hora despues:

```text
WARNING: the Observe collector did not answer at 172.20.0.1:9777 from inside a container.
  Prometheus will start, but the devherd-observe target will stay down.
  if a host firewall is filtering container traffic, allow it: sudo ufw allow from ...
  Fix it, then rerun with --force to rewrite the target.
```

La causa mas comun es el cortafuegos del host filtrando el trafico de los contenedores.
`devherd observe firewall --apply` pone las reglas.

Prometheus es **opcional**: nada del producto depende de que este arrancado.

#### Grafana

`devherd service start grafana` lo levanta con el datasource de Prometheus ya
configurado y un tablero cargado, en `http://127.0.0.1:3000`. **Sin login**: es un
entorno local, el puerto solo escucha en loopback y un login que nadie recuerda es
friccion sin seguridad.

El tablero **DevHerd Observe** trae seis paneles sobre las metricas del collector:
uptime, segundos sin cobertura en 24 h, issues abiertos, eventos por minuto, issues
por proyecto y nivel, y reinicios de contenedor.

Si Prometheus no esta arrancado, el comando lo dice antes:

```text
WARNING: prometheus is not running, so grafana will show empty panels.
  Start it first:  devherd service start prometheus
  Or point grafana at your own prometheus by editing its datasource.
```

No lo arranca solo: levantar contenedores que nadie pidio es peor que avisar, y
puedes tener tu propio Prometheus fuera de DevHerd.

Notas:

- **El datasource apunta a `http://prometheus:9090`**, el alias de red, no una IP.
  Los dos contenedores comparten `infra_net` y Docker resuelve el nombre; una IP
  cambiaria al recrear la red.
- **Las ediciones desde la interfaz sobreviven.** El provisioning declara
  `allowUiUpdates`, y Grafana guarda tus cambios en su propio volumen. Los archivos
  de provisioning tambien respetan tus ediciones, como el resto (`--force` los
  restaura).
- **Mover el tablero fuera de la carpeta `DevHerd`** puede dejarlo inaccesible con
  acceso anonimo. Si pasa, `devherd service start grafana --force` y borrar el
  volumen `devherd_shared_grafana_data` lo devuelve a su sitio.

Notas:

- **El compose es de DevHerd; la configuracion es tuya.** El compose administrado en
  `~/.local/share/devherd/compose/shared-services/docker-compose.yml` se **regenera en
  cada `start` y `stop`**: es el catalogo de lo que DevHerd ofrece, y congelarlo con una
  edicion haria que una version nueva del binario no pudiera ofrecer un servicio nuevo.
  No lo edites, se perdera.
- **Los archivos de configuracion de un servicio si respetan tus ediciones.** Si los
  cambias, DevHerd los deja como estan y avisa por stderr que difieren de su plantilla.
  Con `--force` restaura la plantilla y guarda tu version en un `.bak` al lado. Hoy
  ningun servicio declara configuracion —redis y mailpit no la necesitan— pero la regla
  ya esta puesta.
- `stop` y `status` no crean la red `infra_net`; si la borraste, falla hasta que ejecutes
  un `start`.
- La red de servicios (`infra_net`) y la del proxy (`infra_web`) son **distintas y no se
  conectan solas**. Un proyecto que quiera usar el Redis compartido debe declarar
  `infra_net` en su propio compose.

### 4.17 `devherd observe ...`

Collector local de observabilidad: recibe errores estilo Sentry, los agrupa en *issues* y
ofrece un panel web. Usa una base SQLite separada de la principal. Detalle conceptual en
[observe.md](observe.md).

#### `observe start`

Arranca el collector HTTP **en foreground**. No hay daemon, ni pidfile, ni un comando
`observe stop`: se detiene con Ctrl+C.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | `127.0.0.1:9777` **mas** el gateway de cada red relevante | Direccion de escucha del collector. Si la pasas explicita, solo se usa esa. |

```bash
devherd observe start
# observe collector: http://127.0.0.1:9777
# observe collector: http://172.18.0.1:9777
# observe collector: http://172.20.0.1:9777
# containers on infra_web, infra_net should use http://172.20.0.1:9777

devherd observe start --addr 127.0.0.1:9999   # solo loopback, sin gateways
```

> **Por que varias direcciones.** Dentro de un contenedor `127.0.0.1` es el propio
> contenedor, no el host, asi que un collector solo en loopback no recibe nada de un
> proyecto dockerizado. Y no basta con la red del proxy: a `infra_web` solo se conecta el
> servicio que publica el proxy, no el que reporta. Por eso escucha en el gateway de las
> redes DevHerd y de las de los contenedores ya observados. Con `ufw` activo hace falta
> ademas una regla por red; las da `observe firewall`. Detalle en
> [observe.md](observe.md#alcanzabilidad-desde-contenedores).

> **Es un proceso en foreground.** Si cierras la terminal, los errores se pierden sin rastro.
> Para que sobreviva usa `devherd observe daemon install`.

> **No hay autenticacion.** Las direcciones por defecto son loopback y una subred privada de
> Docker, ninguna expuesta a la LAN. Si cambias `--addr` a `0.0.0.0`, expones la ingesta y el
> panel a toda la red sin ninguna barrera.

#### `observe status [project]`

Consulta `/health` del collector y comprueba que sea alcanzable **desde un contenedor**. Es
un cliente HTTP puro: no necesita `devherd init`.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | `127.0.0.1:9777` | Direccion del collector a consultar. |
| `--check-reachability` | `true` | Sonda el collector desde un contenedor de la red elegida. |

```bash
devherd observe status
devherd observe status aang-server
# observe collector: running at http://127.0.0.1:9777
# status: ok
# container reachability (aang-server on infra_net): ok at http://172.20.0.1:9777
```

**Pasa el proyecto siempre que puedas.** Sin el, la sonda corre en una red compartida
elegida a ciegas y puede devolver un `ok` que no aplica al proyecto que te interesa; con el,
corre en la red que de verdad usan sus contenedores.

La sonda usa la primera imagen que ya tengas en local (`busybox`, `alpine` o
`caddy:2-alpine`) y **nunca descarga ninguna**: si no hay ninguna, imprime el comando
equivalente. Cuando falla, indica si la direccion es loopback y sugiere la regla de
cortafuegos concreta que falta.

#### `observe firewall`

Deriva las reglas que necesita el trafico contenedor -> host, **una por red**, porque cada
una tiene su propia subred de origen y su propio gateway de destino.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | `127.0.0.1:9777` | Direccion del collector que deben permitir las reglas. |
| `--apply` | `false` | Las aplica con `sudo` en vez de solo imprimirlas. |

```bash
devherd observe firewall
# ufw: enabled (rules below are required)
# sudo ufw allow from 172.20.0.0/16 to 172.20.0.1 port 9777 proto tcp comment '...'

devherd observe firewall --apply
```

Detecta si ufw esta activo leyendo `/etc/ufw/ufw.conf`, sin pedir root. `ufw allow` es
idempotente, asi que repetir `--apply` no duplica reglas.

#### `observe daemon install|uninstall|status`

Instala el collector como unidad `systemd --user`, para que arranque al iniciar sesion y se
reinicie si falla, en vez de depender de una terminal abierta.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` (solo `install`) | vacio | Direccion de escucha. Vacio conserva el default (loopback + gateways). |

```bash
devherd observe daemon install
devherd observe daemon status
devherd observe daemon uninstall
```

La unidad se escribe en `$XDG_CONFIG_HOME/systemd/user/devherd-observe.service` y apunta al
binario que ejecuto el comando: si reinstalas DevHerd en otra ruta, vuelve a instalarla.

#### `observe open`

Abre el panel web (`/observe`) en el navegador. No verifica que el collector este vivo.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | `127.0.0.1:9777` | Direccion base del panel. |

```bash
devherd observe open
```

#### `observe dsn <project>`

Imprime el DSN local del proyecto, con formato `http://devherd@<addr>/<project>`. No valida
que el proyecto exista; solo consulta Docker para resolver el gateway de la red compartida, y
si no puede cae a loopback avisando por stderr.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--addr` | gateway de la red compartida | Direccion usada para construir el DSN. |

```bash
devherd observe dsn mi-app
# http://devherd@172.18.0.1:9777/mi-app
```

#### `observe attach <project-or-path> --stack <stack>`

Genera (o previsualiza) un override compose local que inyecta el DSN y la configuracion de
observabilidad en los servicios del proyecto.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--stack` (obligatorio) | — | `laravel`, `node`, `python`, `go`, `docker` o `generic`. |
| `--service` | todos | Servicio(s) compose a observar; repetible o separado por comas. |
| `--environment` | `local` | Valor de `SENTRY_ENVIRONMENT` inyectado. |
| `--addr` | gateway de la red compartida | Direccion usada para construir el DSN por defecto. Si la pasas explicita y es loopback, avisa. |
| `--dsn` | — | Sobrescribe el DSN generado. |
| `--dry-run` | `false` | Previsualiza el override sin escribir archivos. |
| `--reporter` | `false` | Escribe tambien el reporter del proyecto (hoy solo `laravel`). |
| `--force` | `false` | Permite sobrescribir un reporter existente. |

**El override solo inyecta el DSN.** Sin algo que hable con el collector no sale ni un
evento, y esa pieza es codigo dentro de tu proyecto. `--reporter` la genera:

```bash
devherd observe attach mi-app --stack laravel --reporter
# reporter: /ruta/app/Exceptions/DevherdObserveReporter.php
#   wire it up in bootstrap/app.php: ->withExceptions(...)
```

Nunca pisa un archivo existente —es codigo tuyo y puede estar editado— salvo con `--force`.
El reporter expone dos entradas: `report(Throwable $e)` para excepciones y
`capture($type, $message, $context, $level, $fingerprint)` para eventos de dominio que no son
excepciones (un login rechazado, un pago denegado). Detalle en
[guides/observe-laravel.md](guides/observe-laravel.md).

```bash
devherd observe attach mi-app --stack laravel
devherd observe attach mi-app --stack node --service backend --dry-run
devherd observe attach mi-app --stack node --service api,web
```

Variables inyectadas en cada servicio seleccionado: `SENTRY_DSN`, `SENTRY_ENVIRONMENT`,
`DEVHERD_OBSERVE=1`, `DEVHERD_PROJECT` y `DEVHERD_OBSERVE_STACK`. Ademas anade las labels
`devherd.observe`, `devherd.project`, `devherd.service` y `devherd.stack`, que son las que
permiten la correlacion con Docker.

> `attach` **reescribe el archivo completo**, no lo fusiona. Si ejecutas `attach --service
> api` y luego `attach --service web`, solo queda `web`. Para observar varios servicios,
> pasalos todos en la misma invocacion.

> **El `--addr` por defecto genera un DSN inservible en Docker**: `127.0.0.1` dentro de un
> contenedor apunta al contenedor mismo. Usa el mismo `--addr` con el que arrancaste el
> collector (el gateway de `infra_web`), o pasa `--dsn` a mano. Ver
> [observe.md](observe.md#alcanzabilidad-desde-contenedores).

> El override solo surte efecto al **recrear** los contenedores: tras `attach` hace falta un
> `devherd up`.

El override generado se incluye automaticamente en `up`, `stop`, `down` y `logs`.

#### `observe detach <project-or-path>`

Elimina el override de observe del proyecto.

```bash
devherd observe detach mi-app
```

#### `observe scan [project]` / `observe containers [project]`

`scan` toma una instantanea de los contenedores Docker etiquetados como observados y la
guarda; `containers` los lista.

| Comando | Flag | Default |
|---|---|---|
| `observe containers` | `--limit` | `50` |

```bash
devherd observe scan
devherd observe containers --limit 20
```

#### `observe issues [project]` / `observe events [project]` / `observe timeline <event-id>`

Listan issues agrupados, eventos recientes y la linea de tiempo de un evento concreto
(con sus logs de contenedor cercanos y los eventos de contenedor asociados).

| Comando | Flag | Default |
|---|---|---|
| `observe issues` | `--limit` | `20` |
| `observe events` | `--limit` | `20` |

```bash
devherd observe issues
devherd observe events mi-app --limit 50
devherd observe timeline <event-id>
```

`observe timeline` imprime ademas un bloque `Payload:` con los datos que el emisor mando
fuera del modelo normalizado (`context`, `tags`, breadcrumbs...), omitiendo las claves que
ya aparecen como campos propios:

```
Exception: TimbradoFallidoException
Message: El SAT rechazo el timbrado

Payload:
- context: {"cfdi_uuid":"A1B2-C3D4","intento":3,"reintentable":true}
```

> `observe issues` y `observe events` **no filtran por proyecto en el panel web**: la base
> de Observe es unica por maquina y el panel muestra todos los proyectos juntos. El filtro
> por proyecto solo existe en la CLI.

#### `observe alert <add|list|remove|deliveries>`

Reglas de alerta locales.

`observe alert add`:

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--on` (obligatorio) | — | `new-issue`, `error-rate`, `container-exit`, `container-restart`. |
| `--project` | (todas) | Vacio = regla global para cualquier proyecto. |
| `--threshold` | `1` | Solo relevante para `error-rate`. |
| `--window` | `5m` | Duracion estilo Go (`30s`, `5m`, `1h`). Solo para `error-rate`. |
| `--cooldown` | la ventana en `error-rate`, `15m` en el resto | Silencia la regla ese tiempo despues de avisar. `0` entrega siempre. Aplica a los cuatro tipos. |

`observe alert deliveries` acepta `--limit` (default `20`).

```bash
devherd observe alert add --on new-issue --project mi-app
devherd observe alert add --on error-rate --threshold 5 --window 5m
devherd observe alert add --on new-issue --cooldown 30m
devherd observe alert add --on container-restart --cooldown 0   # sin silencio
devherd observe alert list
devherd observe alert remove 1
devherd observe alert deliveries --limit 20
```

> Una "entrega" de alerta es **solo un registro en la base local**. No hay webhooks, ni
> notificaciones del sistema, ni correo. Se consultan con `alert deliveries` o en el panel.
> Cada regla tiene un periodo de enfriamiento (`--cooldown`): despues de avisar se calla
> ese tiempo, de modo que una rafaga produce un aviso y no cincuenta. El umbral se sigue
> evaluando en cada evento; lo que se silencia es la entrega repetida.

#### `observe cleanup`

Elimina datos antiguos de Observe.

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--days` | `14` | Elimina datos mas viejos que N dias. |

```bash
devherd observe cleanup --days 7
```

Borra eventos, logs de contenedor, eventos de contenedor, entregas de alerta e issues.
**No borra el inventario de contenedores observados** ni las reglas de alerta, y no hay
limpieza automatica: hay que ejecutarlo a mano.

### 4.18 `devherd coverage`

Lee un reporte de cobertura ya generado y lo resume en terminal. **DevHerd no
instrumenta codigo ni mide nada**: el reporte lo produce la herramienta de tu stack
y aqui se lee. Detalle completo en [coverage.md](coverage.md).

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--report` | — | Ruta de un reporte ya existente. Obligatorio salvo con `--run`. |
| `--run` | `false` | Prepara el contenedor, corre las pruebas del proyecto y lee el resultado. |
| `--explain` | `false` | Imprime los comandos que `--run` ejecutaria, **sin ejecutar ninguno**. |
| `--stack` | *(detectado)* | Fuerza el stack (`laravel`, `go`). |
| `--service` | *(del stack)* | Servicio compose donde correr las pruebas. |
| `--all` | `false` | Lista todo en vez de lo de mayor masa sin cubrir. |
| `--top` | `10` | Cuantas filas listar cuando no se usa `--all`. |
| `--json` | `false` | Emite el reporte normalizado como JSON. |

Formatos que reconoce, **detectados por el contenido y no por la extension**:

| Formato | Lo genera | Unidad |
|---|---|---|
| `go` | `go test -coverprofile` | sentencias |
| `lcov` | vitest, jest, c8 (Vue, React, TypeScript) | lineas |
| `clover` | PHPUnit | sentencias |
| `jacoco` | Maven, Gradle | lineas |
| `cobertura` | coverage.py | lineas |

Notas:

- **La unidad sale siempre en la cabecera.** Go cuenta sentencias y los demas cuentan
  lineas: un 58% de uno y un 58% de otro **no son comparables**.
- **El total se pondera por unidades**, nunca promediando los porcentajes por archivo.
- **Se ordena por masa sin cubrir**, no por porcentaje, tanto la tabla de
  directorios como la de archivos. Las dos se acotan a `--top`, y si se omiten
  filas se dice cuantas.
- Un reporte sin unidades medibles se reporta como *no coverage data*, no como `0.0%`.

Sobre `--run` (soporta **laravel** y **go**):

- **Instala PCOV si falta** y sube `memory_limit` si esta por debajo de 512M. Los
  dos pasos son idempotentes, y los que ya estaban resueltos se anuncian igual.
- **`-u root` solo cuando el contenedor no lo es.** Las pruebas siempre corren con
  el usuario original.
- **El comando de pruebas se declara** en la seccion `test:` de `.devherd.yml`; sin
  ella se usa el de por defecto del stack y la salida dice cual uso.
- **Si las pruebas fallan**, el aviso va por stderr antes del resumen, el numero se
  muestra igual y se sale con codigo distinto de cero.
- El reporte queda en `.devherd.coverage.*` dentro del proyecto.

### 4.19 `devherd sentry ...`

> **Estado: placeholder.** Este grupo de comandos no realiza ninguna accion funcional. La
> observabilidad real y usable hoy vive en `devherd observe`, que es un subsistema
> independiente.

#### `sentry init <project> --stack <stack>`

| Flag | Default | Descripcion |
|------|---------|-------------|
| `--stack` (obligatorio) | — | `laravel`, `node`, `python` o `go` (el valor no se valida). |
| `--dry-run` | `false` | Imprime un plan de pasos. |

Con `--dry-run` imprime un plan **estatico**: no inspecciona el proyecto ni lee la
configuracion. Sin `--dry-run` devuelve `not implemented`.

```bash
devherd sentry init mi-app --stack laravel --dry-run
```

#### `sentry set-dsn <project> --dsn <dsn>` / `sentry test <project>`

Ambos estan **ocultos** (`Hidden: true`), asi que no aparecen en `devherd sentry --help`.
Siguen siendo invocables y devuelven `not implemented`.

## 5. El manifiesto `.devherd.yml`

Si un proyecto contiene `.devherd.yml`, DevHerd lo usa en lugar de autodetectar el archivo
compose. Formato completo:

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
test:                               # opcional, lo usa `devherd coverage --run`
  command: php artisan test --coverage-clover=.devherd.coverage.xml
  service: app                      # servicio donde correr las pruebas
```

Reglas reales que conviene conocer:

- `compose.files` es **obligatorio y no puede estar vacio**: un manifiesto con la lista
  vacia es un error, no un fallback a autodeteccion.
- Todos los archivos referenciados **deben existir** en el momento de resolver el
  proyecto, y las rutas deben ser relativas a la raiz.
- `proxy.service` y `proxy.port` solo surten efecto **juntos**. Si falta uno de los dos,
  DevHerd cae a las reglas predefinidas por framework, que solo cubren `vue+flask`.
- Declarar `env_file` **desactiva** la carga automatica de `<raiz>/.env` por parte de
  docker compose.
- `version` se parsea pero **nunca se valida**, y las claves desconocidas se ignoran en
  silencio.
- La metadata `proxy` solo la usa el driver `caddy-docker-external`. El driver `caddy` en
  host la ignora por completo.
- `test.command` existe porque **el comando de pruebas no se puede adivinar**: un
  proyecto Laravel con Pest revienta si se le llama `vendor/bin/phpunit`. Sin la
  seccion se usa el de por defecto del stack y `coverage --run` **dice cual uso**.

## 6. Flujos de trabajo

### 6.1 Modo proxy en Docker externo (recomendado)

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

O en un paso, una vez registrado el proyecto:

```bash
devherd serve /home/usuario/develop/examples/hello-vue-flask-docker
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

Recuerda que este driver solo enruta `vue+flask`, `flask` y `vue`.

### 6.3 Repositorio sin Docker

```bash
cd /ruta/a/mi-repo
devherd scaffold --dry-run     # revisa lo que se va a generar
devherd scaffold --db postgres
devherd up
```

### 6.4 Servicios compartidos + observabilidad

```bash
devherd service start redis
devherd service start mailpit
# Mailpit UI: http://127.0.0.1:8025

devherd observe start &           # collector local (foreground; & lo manda a background)
devherd observe attach mi-app --stack node
devherd up /ruta/a/mi-app
devherd observe open
devherd observe issues mi-app
```

### 6.5 Bajar todo

```bash
devherd down /ruta/a/mi-app     # detiene contenedores y limpia el bloque del proxy
devherd service stop redis
devherd service stop mailpit
```

Las entradas de `/etc/hosts` permanecen; hoy no hay comando que las limpie.

## 7. Donde vive el estado

| Que | Ruta (Linux) |
|-----|--------------|
| Config | `~/.config/devherd/config.json` |
| Base de datos principal | `~/.local/share/devherd/devherd.db` |
| Base de datos de Observe | `~/.local/share/devherd/observability/devherd-observe.db` |
| Proxy en host (Caddyfile) | `~/.local/share/devherd/proxy/Caddyfile` |
| Proxy externo (local_proxy) | `~/.local/share/devherd/local_proxy/` |
| Servicios compartidos | `~/.local/share/devherd/compose/shared-services/` |
| Logs / estado | `~/.local/state/devherd/` |
| Override de proxy (por proyecto) | `<proyecto>/.devherd.proxy.override.yml` |
| Override de observe (por proyecto) | `<proyecto>/.devherd.observe.override.yml` |
| Compose generado (por proyecto) | `<proyecto>/docker-compose.devherd.yml` |

En macOS y Windows, el directorio de datos cae bajo `os.UserConfigDir()` y el de estado
bajo `os.UserCacheDir()`, salvo que definas `XDG_DATA_HOME` / `XDG_STATE_HOME`.

Los dos archivos de override y el compose generado se escriben **dentro del repositorio
del proyecto**. Considera anadirlos a tu `.gitignore`; DevHerd no lo hace por ti.

## 8. Patrones recomendados para proyectos Compose

### 8.1 Aislamiento de nombres de contenedor

DevHerd ya aisla los proyectos con un `--project-name` derivado de la ruta absoluta, pero
si tu compose fija `container_name`, ese nombre es global en Docker y colisiona entre
clones. Parametrizalo:

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

Para levantar un clon en paralelo, cambia el prefijo, el dominio, los puertos, la cookie
de sesion y los prefijos de cache/Redis:

```env
COMPOSE_NAME_PREFIX=aang-v2
APP_URL=http://aang-v2.localhost
SESSION_COOKIE=aang_v2_session
CACHE_PREFIX=aang_v2_cache_
REDIS_PREFIX=aang_v2_database_
APP_PORT=8084
FORWARD_DB_PORT=3311
```

`devherd inspect` audita exactamente estas señales y avisa cuando faltan.

### 8.2 Volumenes que sobreviven a cambios de project-name

El nombre por defecto de los volumenes internos deriva del project-name de Compose, asi
que cambiarlo (o clonar el proyecto) crea volumenes nuevos y "pierde" los datos. Para
evitarlo, fija el nombre y parametrizalo:

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

Un volumen ya existente creado por una version anterior se reutiliza declarandolo con su
nombre real y `external: true`.

### 8.3 Probar sin tocar tu configuracion real

Para experimentar sin escribir en tu home, redirige las rutas XDG antes de ejecutar:

```bash
export XDG_CONFIG_HOME=/tmp/devherd-config
export XDG_DATA_HOME=/tmp/devherd-data
export XDG_STATE_HOME=/tmp/devherd-state

devherd init --proxy caddy-docker-external
devherd doctor
```

Ten en cuenta que esto aisla la config y las bases de datos, pero **no** el estado
compartido del sistema: contenedores, redes Docker y `/etc/hosts` siguen siendo globales.
