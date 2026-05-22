# DevHerd: Estado Actual del Proyecto

Este documento resume que ya existe hoy, que ya fue validado en un entorno real y que sigue pendiente.

## 1. Que ya funciona

La CLI ya tiene estos comandos funcionales:

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list`
- `devherd domain set <project> --domain <name>`
- `devherd plan [path]`
- `devherd up [path]`
- `devherd down [path]`
- `devherd proxy apply [project]`
- `devherd open <project>`
- `devherd inspect [path]`
- `devherd sentry init <project> --stack <stack> --dry-run`

Ademas, `plan`, `up` y `down` ya soportan proyectos con manifiesto `.devherd.yml` para definir multiples archivos Compose y un `env_file` opcional.

## 2. Que hace hoy DevHerd

### Inicializacion local

- crea configuracion bajo XDG
- crea la base SQLite local
- guarda preferencias iniciales como proxy, TLD y runtime manager

### Diagnostico del host

- valida rutas locales
- valida Docker CLI
- valida acceso al daemon Docker
- valida `docker compose`
- adapta chequeos al driver de proxy configurado
- en modo `caddy` valida `caddy`, `dnsmasq` y puertos `80` y `443`
- en modo `caddy-docker-external` valida `local_proxy`, `Caddyfile` y puerto `80`

### Registro y deteccion de proyectos

- registra directorios con `park`
- detecta proyectos en la carpeta registrada y en sus hijos inmediatos
- soporta deteccion basica de:
  - Laravel
  - Node.js y Vue
  - Python y Flask
  - Go
  - Docker

### Persistencia

- guarda proyectos registrados en SQLite
- guarda el dominio principal por proyecto
- conserva dominios personalizados al volver a ejecutar `park`

### Dominio principal

- asigna un dominio principal por defecto
- permite cambiarlo con `devherd domain set`

### Proyectos Docker Compose

- inspecciona proyectos con `devherd plan`
- audita colisiones locales con `devherd inspect`
- levanta proyectos con `devherd up`
- baja proyectos con `devherd down`
- soporta `.devherd.yml` con:
  - `compose.files`
  - `compose.env_file`
  - `proxy.domain`
  - `proxy.service`
  - `proxy.port`

### Proxy local

- soporta `caddy` local en host como flujo clasico
- soporta `caddy-docker-external` reutilizando `/home/elyarestark/infra/local_proxy`
- genera `.devherd.proxy.override.yml` para conectar servicios a `infra_web`
- asigna aliases estables por dominio y servicio
- actualiza `/home/elyarestark/infra/local_proxy/Caddyfile`
- recarga el contenedor `infra_caddy`
- `open` usa el dominio efectivo del proyecto o del manifiesto

### Sentry

- reconoce el stack indicado
- muestra el plan de integracion con `--dry-run`
- no modifica archivos todavia en ese modo

## 3. Que ya validamos en un host real

Sobre el proyecto:

```text
/home/elyarestark/develop/examples/hello-vue-flask-docker
```

ya se valido esto:

- `devherd init`
- `devherd doctor`
- `devherd park /home/elyarestark/develop/examples`
- `devherd list`
- `devherd domain set hello-vue-flask-docker --domain mi-demo`
- `devherd up`
- `devherd down`
- `devherd sentry init hello-vue-flask-docker --stack python --dry-run`
- `devherd sentry init hello-vue-flask-docker --stack node --dry-run`

Tambien se valido que:

- el backend Flask responde por `127.0.0.1:8000`
- el frontend Vite responde por `127.0.0.1:5173`
- el proyecto se detecta como `vue+flask`
- el stack se registra como `node+python+docker`

Ademas, ya se valido que:

- `devherd plan /home/elyarestark/develop-work/aang-server`
- resuelve un proyecto sensible con `docker-compose.yml` y `docker-compose.shared.yml`
- detecta `.env` local correctamente
- no requiere levantar contenedores para inspeccionar el stack
- `devherd plan /home/elyarestark/develop-neura/landing-page-neura`
- corrige la autodeteccion y fija `docker-compose.dev.yml` como stack local canonico

## 4. Que ya validamos manualmente con `local_proxy`

Tambien ya se comprobo manualmente el flujo con:

```text
/home/elyarestark/infra/local_proxy
```

usando:

- red Docker compartida `infra_web`
- aliases de red para frontend y backend
- regla manual en el `Caddyfile`
- dominio `http://mi-demo.localhost`

Con eso ya quedo validado end-to-end que:

- `mi-demo.localhost` resuelve correctamente
- Caddy enruta `/` al frontend
- Caddy enruta `/api/*` al backend
- el navegador puede abrir la app
- el frontend puede consumir la API por el dominio del proxy

## 5. Lo que ya quedo automatizado en `2026-05-04`

Sobre la base del flujo manual anterior, DevHerd ya automatiza:

- usar `local_proxy` como driver oficial via `caddy-docker-external`
- usar `.localhost` como TLD por defecto en ese modo
- generar `.devherd.proxy.override.yml`
- conectar servicios a `infra_web`
- crear aliases estables para routing dentro de la red compartida
- escribir y refrescar bloques para dominios administrados dentro del `Caddyfile` externo
- recargar Caddy dentro del contenedor
- resolver `open` contra el dominio efectivo del manifiesto
- leer metadatos `proxy` desde `.devherd.yml`

## 6. Lo que ya validamos en un stack sensible real el `2026-05-04`

Sobre:

```text
/home/elyarestark/develop-work/aang-server
```

ya se valido en real:

- `devherd init --proxy caddy-docker-external`
- `devherd doctor`
- `devherd plan /home/elyarestark/develop-work/aang-server`
- `devherd up /home/elyarestark/develop-work/aang-server`
- `devherd park /home/elyarestark/develop-work/aang-server`
- `devherd proxy apply aang-server`
- `devherd open aang-server`
- `devherd down /home/elyarestark/develop-work/aang-server`

Y durante esa validacion se corrigieron dos huecos:

- `down` dejaba el bloque del dominio en `local_proxy` y el override generado
- `park` podia detectar `node_modules` como proyecto falso

## 7. Validacion operativa de `aang-server` y `Uniformes`

En el entorno real actual se valido el flujo con dos proyectos Laravel dentro de:

```text
/home/elyares/develop/work
```

Proyectos:

- `aang-server`
- `Uniformes`

Quedo validado que:

- ambos proyectos pueden estar arriba al mismo tiempo
- `http://aang.localhost` responde `200 OK`
- `http://uniformes.localhost` responde `200 OK`
- cada proyecto publica su propio bloque en `local_proxy`
- `aang-server` usa `aang_session`
- `Uniformes` usa `uniformes_session`
- `aang-server` usa `CACHE_PREFIX=aang_cache_` y `REDIS_PREFIX=aang_database_`
- `aang-server` conserva su volumen MySQL legado con `DB_VOLUME_NAME=aang-server_aang_db_data` y `DB_VOLUME_EXTERNAL=true`
- `Uniformes` usa `CACHE_PREFIX=uniformes_cache_` y `REDIS_PREFIX=uniformes_database_`
- `devherd inspect` detecta puertos, proxy, `container_name`, Redis/cache/session y volumenes externos

Tambien se aplico el patron de aislamiento inicial en ambos proyectos:

```yaml
container_name: ${COMPOSE_NAME_PREFIX:-aang}_app
```

Y en `.env`:

```env
COMPOSE_NAME_PREFIX=aang
```

Para `Uniformes`, el mismo patron usa:

```env
COMPOSE_NAME_PREFIX=uniformes
```

Esto mantiene los nombres actuales (`aang_app`, `uniformes_app`) pero permite levantar clones cambiando el prefijo y los puertos en `.env`.

## 8. Limitaciones actuales

### Validacion operativa pendiente

Aunque la implementacion y tests ya estan en verde, todavia falta validar en stacks reales sensibles:

- validar el mismo flujo sobre `poderygozo-landing-page`
- validar el entrypoint real de `RetailDataOps`

### Alcance actual del proxy externo

- el flujo externo esta orientado a:
  - proyectos con `proxy.service` y `proxy.port` en `.devherd.yml`
  - fallback `vue+flask` con `backend:8000` y `frontend:5173`
- todavia no existe un contrato universal por framework para generar rutas complejas sin metadatos
- `devherd inspect` ya cubre un primer corte de auditoria de puertos, proxy, `container_name`, Redis/cache/session y volumenes externos
- `devherd up` ejecuta preflight antes de levantar
- `devherd up` aborta si el preflight detecta `FAIL`
- `devherd up --force` permite continuar aunque haya fallos
- `devherd up --no-inspect` permite omitir la auditoria
- DevHerd calcula `--project-name devherd-<slug>-<hash>` desde la ruta absoluta del proyecto
- `down` intenta limpiar tanto el project-name estable como el project-name legado derivado del basename

### Comandos aun no implementados

Siguen pendientes:

- `devherd logs`
- `devherd sentry set-dsn`
- `devherd sentry test`

## 9. En que punto estamos

En este momento DevHerd ya es capaz de:

- inicializar su entorno local
- detectar y registrar proyectos reales
- persistir configuracion y dominios
- levantar servicios compartidos `redis` y `mailpit` via `devherd service`
- crear y validar la red interna compartida `infra_net`
- inspeccionar stacks Compose sin side effects
- auditar colisiones locales con `devherd inspect`
- ejecutar preflight automatico antes de `devherd up`
- usar project-name estable por ruta en comandos Compose
- levantar proyectos Docker Compose
- integrar `local_proxy` Docker externo en el flujo CLI
- preparar la integracion con Sentry en modo seguro

El punto que sigue ya no es diseno del feature. Es ampliacion de validacion operativa sobre proyectos reales y luego compatibilidad por stack.

## 10. Siguiente bloque recomendado

El siguiente bloque de trabajo con mas valor es:

1. validar el mismo flujo sobre `poderygozo-landing-page`
2. confirmar el entrypoint final de `RetailDataOps`
3. documentar patrones de manifiesto por tipo de proyecto
4. reducir dependencia de `container_name` en proyectos reales ahora que el project-name es estable
5. ampliar compatibilidad de routing externo para stacks no `vue+flask`
