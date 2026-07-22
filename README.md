# DevHerd

DevHerd es una plataforma local de desarrollo para Ubuntu inspirada en el flujo de
herramientas como Herd, pero disenada como un producto propio centrado en Linux, servicios
compartidos con Docker y observabilidad local.

Una sola CLI para levantar tus proyectos Docker Compose, publicarlos en dominios locales a
traves de un proxy Caddy, compartir Redis y Mailpit entre ellos, y capturar los errores de
la aplicacion en un panel local.

## Documentacion

| Documento | Para que sirve |
|---|---|
| [docs/USAGE.md](docs/USAGE.md) | **Referencia de comandos**: todos los comandos, flags, ejemplos y flujos. |
| [docs/SYSTEM-OVERVIEW.md](docs/SYSTEM-OVERVIEW.md) | Analisis del sistema: arquitectura, estado medido, limitaciones y deuda. |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Paquetes internos, tipos clave y flujo de datos. |
| [docs/current-status.md](docs/current-status.md) | Estado actual del proyecto y lo que sigue pendiente. |
| [docs/project-workflow.md](docs/project-workflow.md) | Flujos narrativos paso a paso sobre proyectos reales. |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | Como compilar, testear y anadir comandos. |
| [docs/observe.md](docs/observe.md) | Observabilidad local en detalle. |
| [docs/guides/scaffold.md](docs/guides/scaffold.md) | Generacion de `docker-compose` para repos sin contenedores. |
| [docs/guides/logging-and-logs.md](docs/guides/logging-and-logs.md) | Logging de diagnostico y comando `logs`. |
| [docs/guides/vikunja.md](docs/guides/vikunja.md) | Ejemplo completo con Vikunja. |
| [docs/technical-plan.md](docs/technical-plan.md) | Plan tecnico original del producto. |
| [docs/IMPROVEMENTS.md](docs/IMPROVEMENTS.md) | Revision de arquitectura e infraestructura, con roadmap. |

## Estado actual

DevHerd esta en **MVP/alpha** (`0.1.0-alpha`) y es Ubuntu/Linux-first.

Lo que ya funciona:

- **Ciclo de vida Compose**: `up`, `stop`, `down`, `logs` y `plan`, con nombre de proyecto
  estable por ruta para poder levantar clones del mismo repo a la vez.
- **Preflight automatico**: `inspect` audita colisiones de puertos, `container_name`,
  volumenes externos y configuracion tipo Laravel; `up` lo ejecuta antes de arrancar.
- **Proxy local** en dos modos: `caddy-docker-external` (contenedor administrado, el
  recomendado) y `caddy` en host.
- **Dominios locales** por proyecto, con bloque administrado en `/etc/hosts`.
- **Scaffold**: genera `docker-compose` y manifiesto para repos sin contenedores
  (`vue+flask`, `laravel`, `vue`, `flask`, `node`, `go`), con menu de base de datos, Redis
  y puertos sin colision. `up` lo ofrece automaticamente cuando no hay compose.
- **Servicios compartidos**: Redis y Mailpit en la red `infra_net`.
- **Observabilidad local** (`observe`): collector propio, panel web, base SQLite separada,
  DSN local, attach/detach por proyecto, correlacion con contenedores Docker, issues,
  eventos, timeline, alertas locales y limpieza de datos.
- **`serve`**: encadena `up` + `proxy apply` + `open` en un solo comando.
- **Diagnostico**: `doctor` valida los prerequisitos del host; los flags globales
  `--verbose` y `--log-json` emiten trazas estructuradas a stderr.

Limitaciones principales a dia de hoy:

- El driver `caddy` en host solo enruta `vue+flask`, `flask` y `vue`, e ignora la metadata
  `proxy` del manifiesto. El modo recomendado es `caddy-docker-external`.
- `devherd sentry` es un placeholder: la observabilidad usable vive en `devherd observe`.
- El collector de `observe` no descomprime envelopes gzip, lo que limita el uso de SDKs
  Sentry oficiales sin configuracion adicional.

El detalle completo, con metricas medidas, esta en
[docs/SYSTEM-OVERVIEW.md](docs/SYSTEM-OVERVIEW.md) y
[docs/current-status.md](docs/current-status.md).

## Quickstart

```bash
# 1. Inicializa DevHerd (modo recomendado)
go run ./cmd/devherd init --proxy caddy-docker-external
go run ./cmd/devherd doctor

# 2. Registra tu carpeta de proyectos
go run ./cmd/devherd park ~/develop/examples

# 3. Revisa un proyecto antes de levantarlo
go run ./cmd/devherd plan ~/develop/examples/mi-app
go run ./cmd/devherd inspect ~/develop/examples/mi-app

# 4. Levantalo y publicalo
go run ./cmd/devherd up ~/develop/examples/mi-app
go run ./cmd/devherd proxy apply mi-app
go run ./cmd/devherd open mi-app

go run ./cmd/devherd list
```

Una vez registrado el proyecto, los tres pasos finales se reducen a uno:

```bash
go run ./cmd/devherd serve ~/develop/examples/mi-app
```

## Instalacion

Durante desarrollo, ejecuta `go run ./cmd/devherd <comando>` desde la raiz del repositorio.

Para usar `devherd` desde cualquier carpeta:

```bash
./scripts/install-ubuntu.sh        # compila e instala en ~/.local/bin/devherd
./scripts/install-caddy-ubuntu.sh  # opcional, solo para el modo de proxy en host
devherd --help
```

Para tener metadatos de version reales (commit y fecha), compila con el `Makefile`:

```bash
make build      # bin/devherd con version + commit + fecha
make install    # go install en $GOBIN
```

Los comandos que operan sobre proyectos aceptan una ruta explicita, asi que puedes
invocarlos desde cualquier directorio:

```bash
devherd up ~/develop/examples/mi-app
devherd proxy apply mi-app
devherd open mi-app
```

## Requisitos

- Docker con `docker compose` (plugin v2) y engine Linux.
- Para el driver `caddy` en host: binario `caddy`, puertos 80/443 libres y acceso `sudo`.
- Para el driver `caddy-docker-external`: solo Docker.

`devherd doctor` valida todo esto y adapta sus chequeos al driver configurado.
