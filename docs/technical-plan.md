# DevHerd Ubuntu: Plan Tecnico Inicial

## 1. Vision del producto

DevHerd debe ser una plataforma Ubuntu-first para desarrollo local con tres objetivos:

1. Registrar y descubrir proyectos de desarrollo de forma consistente.
2. Exponerlos con dominios locales `.test` y proxy inverso administrado.
3. Conectar cada proyecto con observabilidad basada en SDKs oficiales de Sentry.

No busca clonar Laravel Herd ni Sentry. Busca combinar una experiencia local profesional con una capa opinionada de automatizacion para Linux.

## 2. Como funcionaria

Flujo operativo esperado:

1. El usuario ejecuta `devherd init`.
2. DevHerd crea su estructura local, archivo de configuracion y base SQLite.
3. El usuario ejecuta `devherd doctor` para validar Docker, Caddy, `dnsmasq`, puertos y permisos locales.
4. El usuario registra uno o varios directorios con `devherd park ~/Code`.
5. El detector recorre los directorios y clasifica proyectos Laravel, Node.js, Go, Python o Docker.
6. Por cada proyecto registrado, el gestor de dominios asigna `proyecto.test` como dominio principal.
7. El modulo de proxy renderiza templates de Caddy y recarga el servicio.
8. `dnsmasq` resuelve `*.test` a `127.0.0.1`.
9. El gestor de servicios levanta Redis y Mailpit con Docker Compose como primer corte.
10. El modulo de Sentry detecta el stack y ejecuta `devherd sentry init <project> --stack <stack> --dry-run` antes de tocar archivos del proyecto.
11. El usuario inspecciona el estado con `devherd list`, `devherd logs`, `devherd service status` y `devherd sentry test`.

## 3. Requerimientos funcionales

### Core platform

- Inicializar el entorno local de DevHerd en Ubuntu.
- Verificar prerequisitos del host con `devherd doctor`.
- Registrar uno o varios directorios "parked".
- Detectar proyectos dentro de esos directorios.
- Clasificar stack y framework por proyecto.
- Guardar proyectos, dominios, servicios y preferencias en SQLite.
- Asignar dominios locales `.test`.
- Soportar solo dominio principal por proyecto en MVP 1.
- Generar configuracion de reverse proxy.
- Integrar resolucion local DNS para `.test`.
- Abrir proyectos por nombre desde CLI.
- Ver logs del proyecto y de servicios asociados.

### Servicios compartidos

- Iniciar, detener y consultar servicios compartidos.
- Limitar MVP 1 a Redis y Mailpit.
- Mantener templates Compose reutilizables.
- Exponer puertos y credenciales por defecto seguras para entorno local.
- Persistir estado y configuracion por servicio.

### Runtimes

- Detectar versiones requeridas por proyecto.
- Integrarse con `mise` o `asdf`.
- Resolver runtime preferido por proyecto.

### Observabilidad y Sentry

- Configurar un DSN por proyecto.
- Inicializar Sentry segun stack.
- Ejecutar `--dry-run` antes de instalar dependencias o modificar archivos.
- Instalar SDK oficial por framework/lenguaje.
- Escribir configuracion minima en `.env` o equivalente.
- Ejecutar envio de evento de prueba.
- Limitar MVP 1 a Sentry Cloud.

### UX CLI

- Comandos consistentes, idempotentes y con salida clara.
- Mensajes accionables en errores de permisos, puertos o dependencias faltantes.
- Modo no interactivo para CI/local automation cuando aplique.

## 4. Requerimientos no funcionales

### Plataforma

- Ubuntu 22.04+ como target primario.
- Operacion local sin depender de un backend remoto propio.
- Compatibilidad con Docker Engine y Docker Compose plugin.

### Calidad

- Arranque rapido de CLI.
- Persistencia robusta ante reinicios.
- Operaciones idempotentes para `init`, `park` y render de proxy.
- Trazabilidad de eventos locales.

### Mantenibilidad

- Arquitectura modular por dominios.
- Templates externos para proxy, Compose y Sentry.
- Separacion clara entre deteccion, persistencia y provisionamiento.

### Operacion

- Logs estructurados para agente y CLI.
- Configuracion bajo directorios XDG.
- Posibilidad futura de ejecutar un agente en background.
- MVP 1 sin daemon local ni app desktop.

## 5. Arquitectura del sistema

Arquitectura propuesta por capas:

1. CLI:
   Cobra como interfaz principal (`devherd init`, `doctor`, `park`, `service`, `sentry`).
2. Application layer:
   Casos de uso como `InitEnvironment`, `ParkDirectory`, `StartService`, `InitSentry`.
3. Domain modules:
   `doctor`, `detector`, `proxy`, `dns`, `services`, `sentry`, `logs`, `runtimes`.
4. Infrastructure:
   SQLite, archivos de configuracion, templates, Docker CLI, Caddy, dnsmasq.

### Componentes principales

- `internal/doctor`: verificacion del host y prerequisitos del MVP.
- `internal/config`: rutas XDG, carga y escritura de configuracion.
- `internal/database`: migraciones y acceso SQLite.
- `internal/detector`: deteccion de stacks/frameworks.
- `internal/proxy`: generacion y reload de Caddy.
- `internal/dns`: resolucion local inicial via `/etc/hosts`, con dnsmasq como siguiente paso.
- `internal/services`: definiciones Compose y estado de servicios.
- `internal/sentry`: bootstrap SDK y test event.
- `internal/logs`: agregacion y tail de logs.
- `internal/runtimes`: integracion con mise/asdf.
- `internal/api`: futura capa para app desktop o daemon local, fuera de MVP 1.

### Decisiones iniciales recomendadas

- Proxy inicial: Caddy.
- DNS local final: dnsmasq.
- Resolucion local inicial: bloque administrado en `/etc/hosts`.
- Runtime manager default: mise.
- Base local: SQLite.
- Driver SQLite en Go: `modernc.org/sqlite` para evitar dependencia de CGO.
- Sentry target inicial: Sentry Cloud con SDKs oficiales.
- Servicios iniciales: Redis y Mailpit.
- Sin daemon ni desktop app en MVP 1.

## 6. Modelo de datos inicial en SQLite

### Tabla `settings`

- `key` TEXT PRIMARY KEY
- `value` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### Tabla `parks`

- `id` INTEGER PRIMARY KEY
- `path` TEXT NOT NULL UNIQUE
- `created_at` TEXT NOT NULL

### Tabla `projects`

- `id` INTEGER PRIMARY KEY
- `name` TEXT NOT NULL UNIQUE
- `path` TEXT NOT NULL UNIQUE
- `stack` TEXT NOT NULL
- `framework` TEXT NOT NULL
- `runtime` TEXT NOT NULL
- `status` TEXT NOT NULL
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### Tabla `project_domains`

- `id` INTEGER PRIMARY KEY
- `project_id` INTEGER NOT NULL
- `domain` TEXT NOT NULL UNIQUE
- `kind` TEXT NOT NULL
- `is_primary` INTEGER NOT NULL
- `created_at` TEXT NOT NULL

### Tabla `runtime_preferences`

- `id` INTEGER PRIMARY KEY
- `project_id` INTEGER NOT NULL
- `runtime` TEXT NOT NULL
- `version` TEXT NOT NULL
- `manager` TEXT NOT NULL
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### Tabla `services`

- `id` INTEGER PRIMARY KEY
- `name` TEXT NOT NULL UNIQUE
- `driver` TEXT NOT NULL
- `status` TEXT NOT NULL
- `compose_project` TEXT NOT NULL
- `config_json` TEXT NOT NULL
- `started_at` TEXT
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### Tabla `sentry_configs`

- `id` INTEGER PRIMARY KEY
- `project_id` INTEGER NOT NULL UNIQUE
- `provider` TEXT NOT NULL
- `dsn` TEXT NOT NULL
- `environment` TEXT NOT NULL
- `sample_rate` REAL NOT NULL
- `enabled` INTEGER NOT NULL
- `created_at` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### Tabla `events`

- `id` INTEGER PRIMARY KEY
- `scope` TEXT NOT NULL
- `scope_id` INTEGER
- `level` TEXT NOT NULL
- `message` TEXT NOT NULL
- `meta_json` TEXT NOT NULL
- `created_at` TEXT NOT NULL

## 7. Estructura de carpetas

```text
devherd/
├── cmd/devherd/
├── docs/
├── internal/
│   ├── api/
│   ├── cli/
│   ├── config/
│   ├── database/
│   ├── detector/
│   ├── doctor/
│   ├── dns/
│   ├── logs/
│   ├── proxy/
│   ├── runtimes/
│   ├── sentry/
│   ├── services/
│   └── version/
├── apps/desktop/
├── templates/
│   ├── caddy/
│   ├── docker/
│   ├── nginx/
│   └── sentry/
└── scripts/
```

## 8. Diseño de comandos CLI

### MVP inicial

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list`
- `devherd open <project>`
- `devherd logs <project>`
- `devherd service start redis|mailpit`
- `devherd service stop redis|mailpit`
- `devherd service status [redis|mailpit]`
- `devherd sentry init <project> --stack <stack> --dry-run`
- `devherd sentry set-dsn <project> --dsn <dsn>`
- `devherd sentry test <project>`

### Opciones futuras

- `devherd proxy reload`
- `devherd domains refresh`
- `devherd runtimes use <project>`
- `devherd ui`

## 9. Roadmap por fases

### Fase 1: Bootstrap

- CLI con Cobra.
- Configuracion XDG.
- SQLite y migraciones.
- `devherd init`.
- `devherd doctor`.

### Fase 2: Descubrimiento

- Registro de parks.
- Detector Laravel, Node.js, Go, Python y Docker.
- Persistencia de proyectos.
- `devherd park` y `devherd list`.

### Fase 3: Networking local

- Soporte `dnsmasq`.
- Render de Caddy.
- Recarga de proxy.
- Solo dominio principal `proyecto.test`.
- `devherd open`.
- Primer corte implementable con `/etc/hosts` antes de cerrar el modulo `dnsmasq`.

### Fase 4: Servicios compartidos

- Templates Compose.
- Ciclo de vida inicial de Redis y Mailpit.
- `devherd service start/stop/status`.
- Expansion posterior a PostgreSQL, MySQL, MinIO y Adminer.

### Fase 5: Sentry

- Configuracion DSN por proyecto.
- Bootstrap por stack.
- Modo `--dry-run` antes de cambios reales.
- Evento de prueba.

### Fase 6: Logs y DX

- Tail de logs por proyecto.
- Logs de proxy y contenedores.
- `devherd logs`.

### Fase 7: Desktop

- Tauri + Vue 3 + TailwindCSS.
- Dashboard, servicios, logs y Sentry.
- Fase explicitamente fuera de MVP 1.

## 10. Primer MVP alcanzable

Un MVP realista en 2 iteraciones pequenas seria:

### MVP A

- `devherd init`
- `devherd doctor`
- `devherd park`
- `devherd list`
- detector basico para Laravel, Node.js y Docker
- persistencia SQLite
- validacion de Docker, Caddy, `dnsmasq`, puertos y permisos locales

### MVP B

- Caddy generado automaticamente
- dominio principal `proyecto.test`
- `devherd open`
- `devherd service start redis|mailpit`
- `devherd sentry init <project> --stack laravel --dry-run`

Ese corte ya entrega valor real sin esperar toda la plataforma.

## 11. Riesgos tecnicos

- Permisos de sistema para manipular `dnsmasq`, puertos 80/443 y reload de proxy.
- Diferencias entre layouts de proyectos reales.
- Colisiones de dominios y puertos.
- Ambiguedad al detectar stacks mixtos.
- Dependencia del estado de Docker en el host.
- Instalacion automatica de SDKs Sentry que pueda romper lockfiles o convenciones del proyecto.
- Falsos positivos en chequeos del host si el usuario ya tiene puertos ocupados por herramientas compatibles.
- Soporte de multiples versiones de runtimes en equipos heterogeneos.
- Observabilidad parcial si un stack requiere configuracion manual adicional.

## 12. Buenas practicas de seguridad

- No ejecutar procesos como root salvo operaciones estrictamente necesarias.
- Guardar secretos fuera del repo y con permisos restrictivos.
- Validar rutas antes de generar configuraciones o ejecutar comandos.
- Escapar parametros en templates y comandos shell.
- Registrar eventos administrativos sin exponer DSNs completos.
- Aplicar principle of least privilege a archivos de configuracion y sockets.
- Mantener allowlists de servicios soportados en lugar de ejecutar Compose arbitrario.
- Confirmar operaciones destructivas futuras como `unlink`, `unpark` o `purge`.
- Separar siempre `dry-run` de `apply` en integraciones que modifiquen dependencias o archivos fuente.

## 13. Mejoras recomendadas

- Separar "CLI orchestration" de "agent/daemon" desde el inicio mediante interfaces, aunque el daemon llegue despues.
- Crear un comando `doctor` temprano para verificar Docker, Caddy, dnsmasq, mise y permisos.
- Diseñar el detector como pipeline extensible con prioridades y confianza.
- Tratar subdominios como recurso declarativo en SQLite, no como strings derivadas solo en runtime.
- Añadir un sistema de eventos internos para auditar acciones como `park`, `proxy render` y `sentry init`.
- Mantener los templates versionados para poder migrarlos sin romper instalaciones existentes.
- Mantener el MVP intencionalmente corto: Caddy, `dnsmasq`, Redis, Mailpit, dominio principal y Sentry Cloud.

## 14. Base inicial incluida en este repositorio

El scaffold inicial implementa:

- CLI con Cobra.
- Comando `devherd init`.
- Comando `devherd doctor`.
- Comandos `devherd park` y `devherd list` con deteccion basica y persistencia SQLite.
- Comandos `devherd proxy apply` y `devherd open`.
- Comandos `devherd up` y `devherd down` para proyectos con Compose.
- Configuracion JSON local en XDG.
- Creacion de directorios locales de DevHerd.
- Inicializacion de SQLite con el modelo de datos base.
- Templates iniciales y stubs para el resto del roadmap.
- Contrato inicial de `devherd sentry init <project> --stack <stack> --dry-run`.
