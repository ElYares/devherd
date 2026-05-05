# DevHerd

DevHerd es una plataforma local de desarrollo para Ubuntu inspirada en el flujo de herramientas como Herd, pero diseñada como un producto propio centrado en Linux, servicios compartidos con Docker y observabilidad con Sentry.

## Estado actual

- CLI inicial en Go con Cobra.
- Configuracion local basada en XDG.
- Base SQLite inicial para proyectos, dominios, servicios y configuracion de Sentry.
- Comando `devherd init` implementado.
- Comando `devherd doctor` implementado para validar prerequisitos del MVP.
- Comandos `park` y `list` implementados con deteccion basica de proyectos.
- Comando `domain set` implementado para personalizar el dominio principal.
- Comando `plan` implementado para inspeccionar stacks Compose sin side effects.
- Comando `proxy apply` implementado tanto para Caddy local en host como para `local_proxy` Docker externo.
- Comandos `up` y `down` implementados para proyectos con `docker-compose`, incluyendo manifiesto `.devherd.yml`.
- `devherd sentry init <project> --stack <stack> --dry-run` implementado.
- Comando `open` implementado para abrir el dominio del proyecto en el navegador.
- `logs`, `service`, `sentry set-dsn` y `sentry test` siguen como siguiente iteracion.

## Enfoque del MVP 1

- `doctor` temprano para validar Docker, proxy activo y escritura local.
- Proxy soportado hoy:
  - `caddy` en host con resolucion local via `/etc/hosts`
  - `caddy-docker-external` reutilizando `/home/elyarestark/infra/local_proxy`
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
El flujo de uso por proyecto vive en [docs/project-workflow.md](docs/project-workflow.md).
El estado actual del proyecto vive en [docs/current-status.md](docs/current-status.md).
