# DevHerd: Estado Actual del Proyecto

Este documento resume que ya existe hoy, que ya fue validado en un entorno real y que sigue pendiente.

## 1. Que ya funciona

La CLI ya tiene estos comandos funcionales:

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list`
- `devherd domain set <project> --domain <name>`
- `devherd up [path]`
- `devherd down [path]`
- `devherd proxy apply [project]`
- `devherd open <project>`
- `devherd sentry init <project> --stack <stack> --dry-run`

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
- valida `caddy`
- valida `dnsmasq`
- valida disponibilidad de puertos `80` y `443`

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

- levanta proyectos con `devherd up`
- baja proyectos con `devherd down`

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

## 5. Lo que todavia no esta automatizado

Aunque el flujo con `local_proxy` ya funciona manualmente, DevHerd todavia no lo automatiza.

Hoy todavia falta:

- usar `local_proxy` como driver oficial de proxy
- usar `.localhost` como TLD natural en ese modo
- generar automaticamente un `docker-compose.override.yml`
- conectar automaticamente el proyecto a `infra_web`
- crear aliases estables para frontend y backend
- escribir un bloque administrado dentro del `Caddyfile` externo
- recargar automaticamente el contenedor `caddy`

## 6. Limitaciones actuales

### Proxy actual de DevHerd

El `proxy apply` actual esta pensado para:

- Caddy instalado en el host
- sincronizacion de dominios en `/etc/hosts`
- recarga de `caddy` local con `sudo`

Eso significa que hoy:

- `devherd proxy apply` no usa tu `local_proxy` en Docker
- `devherd open` depende del dominio registrado, pero no resuelve por si solo el flujo Docker externo

### Comandos aun no implementados

Siguen pendientes:

- `devherd logs`
- `devherd service start|stop|status`
- `devherd sentry set-dsn`
- `devherd sentry test`

## 7. En que punto estamos

En este momento DevHerd ya es capaz de:

- inicializar su entorno local
- detectar y registrar proyectos reales
- persistir configuracion y dominios
- levantar proyectos Docker Compose
- preparar la integracion con Sentry en modo seguro

Y ademas ya sabemos que el flujo objetivo con proxy Docker externo funciona en la practica, aunque hoy siga siendo manual.

## 8. Siguiente bloque recomendado

El siguiente bloque de trabajo con mas valor es:

1. integrar `local_proxy` como driver `caddy-docker-external`
2. soportar `.localhost` como TLD por defecto en ese modo
3. generar un override de Compose administrado por DevHerd
4. escribir y mantener un bloque administrado en el `Caddyfile` externo
5. recargar automaticamente el contenedor `caddy`
6. hacer que `devherd open` abra directamente `http://mi-demo.localhost`
