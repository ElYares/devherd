# DevHerd

DevHerd es una plataforma local de desarrollo inspirada en el flujo de herramientas como Herd, pero diseñada como un producto propio centrado en Docker, servicios compartidos y observabilidad local. Linux sigue siendo el entorno principal; Windows nativo y WSL2 usan el proxy Docker externo.

## Estado actual

- CLI inicial en Go con Cobra.
- Configuracion local portable basada en directorios del usuario.
- Base SQLite inicial para proyectos, dominios, servicios y configuracion de Sentry.
- Comando `devherd init` implementado.
- Comando `devherd doctor` implementado para validar prerequisitos del MVP.
- Comandos `park` y `list` implementados con deteccion basica de proyectos.
- Comando `domain set` implementado para personalizar el dominio principal.
- Comando `plan` implementado para inspeccionar stacks Compose sin side effects.
- Comando `inspect` implementado para detectar colisiones locales antes o despues de levantar un stack.
- `devherd up` ejecuta preflight automaticamente y soporta `--force` y `--no-inspect`.
- Los comandos Compose usan `--project-name` estable por ruta para aislar clones con el mismo nombre de carpeta.
- Comando `proxy apply` implementado tanto para Caddy local en host como para `local_proxy` Docker externo administrado por DevHerd.
- Comando `proxy bootstrap` implementado para crear o reparar los assets del proxy externo.
- Comandos `up`, `stop` y `down` implementados para proyectos con `docker-compose`, incluyendo manifiesto `.devherd.yml`.
- Comandos `service start|stop|status` implementados para Redis y Mailpit compartidos.
- `devherd observe` implementado con collector local, panel web, SQLite separada, DSN local, attach/detach por proyecto, correlacion Docker, logs cercanos, issues/eventos, alertas locales y limpieza de datos viejos.
- `devherd sentry init <project> --stack <stack> --dry-run` implementado.
- Comando `open` implementado para abrir el dominio del proyecto en el navegador.
- `logs`, `sentry set-dsn` y `sentry test` siguen como siguiente iteracion.

## Enfoque del MVP 1

- `doctor` temprano para validar Docker, proxy activo y escritura local.
- Proxy soportado hoy:
  - `caddy` en host con resolucion local via `/etc/hosts`
  - `caddy-docker-external` usando un `local_proxy` administrado bajo el data dir de DevHerd, por defecto `~/.local/share/devherd/local_proxy`
- Dominio principal por proyecto:
  - `proyecto.test` en modo host
  - `proyecto.localhost` en modo `caddy-docker-external`
- Solo Redis y Mailpit como servicios compartidos iniciales.
- Solo Sentry Cloud como proveedor inicial.
- `devherd sentry init` con `--dry-run` antes de modificar archivos del proyecto.
- Sin daemon ni app desktop en MVP 1.

## Quickstart

```bash
go mod tidy
go run ./cmd/devherd init --proxy caddy-docker-external
go run ./cmd/devherd doctor
go run ./cmd/devherd park /home/elyarestark/develop/examples
go run ./cmd/devherd plan /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd inspect /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd domain set hello-vue-flask-docker --domain mi-demo
go run ./cmd/devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
go run ./cmd/devherd proxy apply hello-vue-flask-docker
go run ./cmd/devherd open hello-vue-flask-docker
go run ./cmd/devherd list
```

## Desde donde ejecutar

Durante desarrollo:

- Ejecuta `go run ./cmd/devherd <comando>` desde la raiz del repositorio.

Si quieres usar `devherd` desde cualquier carpeta:

```bash
./scripts/install-ubuntu.sh
./scripts/install-caddy-ubuntu.sh
devherd --help
```

En Windows nativo, desde PowerShell:

```powershell
.\scripts\install-windows.ps1 -AddToPath
devherd init
devherd doctor
```

En Windows, `devherd init` usa `caddy-docker-external` y dominios `.localhost` por defecto.

Una vez instalado en `~/.local/bin/devherd`, puedes ejecutar la CLI desde cualquier directorio. Los comandos que operan sobre proyectos aceptan ruta explicita, por ejemplo:

```bash
devherd park /home/elyarestark/develop/examples
devherd plan /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker
```

El plan tecnico completo vive en [docs/technical-plan.md](docs/technical-plan.md).
La referencia de comandos vive en [docs/cli-commands.md](docs/cli-commands.md).
La guia de Windows vive en [docs/windows.md](docs/windows.md).
El flujo de uso por proyecto vive en [docs/project-workflow.md](docs/project-workflow.md).
El estado actual del proyecto vive en [docs/current-status.md](docs/current-status.md).
El plan de observabilidad local vive en [docs/observe.md](docs/observe.md).
