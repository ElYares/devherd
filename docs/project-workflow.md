# DevHerd: Flujo de Uso por Proyecto

Esta guia describe como usar DevHerd sobre un proyecto real y separa con claridad dos cosas:

- El flujo que ya funciona hoy en el MVP actual.
- El flujo objetivo para integrar un proxy Docker externo como `local_proxy`.

## 1. Alcance

Esta guia usa como ejemplo un proyecto compuesto `Vue + Flask + Docker Compose` ubicado en:

```text
/home/elyarestark/develop/examples/hello-vue-flask-docker
```

El directorio padre que se registra con `park` es:

```text
/home/elyarestark/develop/examples
```

## 2. Estado real de la CLI

### Funciona hoy

- `devherd init`
- `devherd doctor`
- `devherd park <path>`
- `devherd list`
- `devherd domain set <project> --domain <name>`
- `devherd plan [path]`
- `devherd inspect [path]`
- `devherd up [path]`
- `devherd down [path]`
- `devherd proxy apply [project]`
- `devherd open <project>`
- `devherd sentry init <project> --stack <stack> --dry-run`

### Todavia no esta listo

- `devherd logs`
- `devherd service start|stop|status`
- `devherd sentry set-dsn`
- `devherd sentry test`
- Integracion nativa con `local_proxy` como driver externo

## 3. Prerequisitos

Para el flujo actual del MVP:

- Ubuntu
- Docker
- Docker Compose plugin
- Binario `devherd` instalado

Si quieres usar `proxy apply` y `open` sobre dominios locales hoy, ademas necesitas:

- Caddy instalado en el host
- permisos para escribir el bloque administrado en `/etc/hosts`

## 4. Instalacion del binario

Desde la raiz del repo:

```bash
./scripts/install-ubuntu.sh
```

Luego valida:

```bash
devherd --help
```

## 5. Flujo actual de uso en un proyecto

### Paso 1. Inicializar DevHerd

```bash
devherd init
```

Esto crea:

- `~/.config/devherd/config.json`
- `~/.local/share/devherd/devherd.db`

### Paso 2. Validar el host

```bash
devherd doctor
```

Debes revisar especialmente:

- acceso al daemon Docker
- `docker compose`
- `caddy`
- puertos `80` y `443`

Si `caddy` no existe, puedes seguir usando `park`, `list`, `up`, `down` y `sentry --dry-run`, pero el dominio local no se va a publicar en navegador.

### Paso 3. Registrar una carpeta de proyectos

```bash
devherd park /home/elyarestark/develop/examples
```

DevHerd hace esto:

- guarda la carpeta como `park`
- revisa esa ruta y sus hijos inmediatos
- detecta stacks compatibles
- registra cada proyecto en SQLite
- asigna un dominio principal por defecto

En el ejemplo, el proyecto detectado es:

- `hello-vue-flask-docker`

### Paso 4. Verificar los proyectos registrados

```bash
devherd list
```

Debes ver columnas como:

- `NAME`
- `FRAMEWORK`
- `STACK`
- `DOMAIN`
- `STATUS`
- `PATH`

### Paso 5. Personalizar el dominio principal

```bash
devherd domain set hello-vue-flask-docker --domain mi-demo
```

Con la configuracion actual, DevHerd lo normaliza como:

```text
mi-demo.test
```

Luego puedes confirmar:

```bash
devherd list
```

### Paso 6. Levantar el proyecto

Antes de levantar un stack real, puedes inspeccionarlo sin side effects:

```bash
devherd plan /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Si ya estas dentro del proyecto:

```bash
devherd plan
```

Esto imprime la raiz resuelta, los archivos Compose activos, el `.env` detectado y el comando base de `docker compose`.

Para auditar colisiones reales antes de levantar o despues de levantar, usa:

```bash
devherd inspect /home/elyarestark/develop/examples/hello-vue-flask-docker
```

`inspect` revisa puertos, `container_name`, estado del proxy, variables de sesion/cache/Redis y volumenes externos.

Despues ya puedes levantar el proyecto:

Si estas fuera de la carpeta del proyecto:

```bash
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Si ya estas dentro del proyecto:

```bash
devherd up
```

Esto ejecuta `docker compose up` sobre ese proyecto.

Si el proyecto define un archivo `.devherd.yml`, DevHerd usa los archivos listados en `compose.files` en lugar de asumir un unico `docker-compose.yml`.

Ejemplo:

```yaml
version: 1
compose:
  files:
    - docker-compose.yml
    - docker-compose.shared.yml
```

### Paso 7. Validar la app por puertos directos

Antes de depender del proxy, conviene validar la aplicacion directamente:

```bash
curl http://127.0.0.1:8000/api/health
curl http://127.0.0.1:8000/api/message
curl http://127.0.0.1:5173
```

En este ejemplo:

- Flask responde en `8000`
- Vite responde en `5173`

Para validar la UI completa, abre en navegador:

```text
http://127.0.0.1:5173
```

### Paso 8. Publicar el dominio local con Caddy

Este paso solo aplica si tienes Caddy instalado en el host y `doctor` ya no marca fallo.

```bash
devherd proxy apply hello-vue-flask-docker
```

Ese comando hoy hace esto:

- genera configuracion de Caddy
- sincroniza dominios administrados en `/etc/hosts`
- valida la configuracion
- recarga o arranca Caddy

### Paso 9. Abrir el proyecto por nombre

```bash
devherd open hello-vue-flask-docker
```

Eso intenta abrir el dominio principal del proyecto en tu navegador.

### Paso 10. Probar el flujo de Sentry sin tocar archivos

Backend Python:

```bash
devherd sentry init hello-vue-flask-docker --stack python --dry-run
```

Frontend Node:

```bash
devherd sentry init hello-vue-flask-docker --stack node --dry-run
```

El objetivo de este paso es:

- confirmar que DevHerd reconoce el stack
- ver el plan de cambios
- no modificar el proyecto todavia

### Paso 11. Apagar el proyecto

Si estas fuera de la carpeta:

```bash
devherd down /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Si estas dentro del proyecto:

```bash
devherd down
```

## 6. Flujo recomendado hoy para el proyecto de ejemplo

```bash
devherd init
devherd doctor
devherd park /home/elyarestark/develop/examples
devherd list
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
curl http://127.0.0.1:8000/api/health
curl http://127.0.0.1:8000/api/message
curl http://127.0.0.1:5173
devherd sentry init hello-vue-flask-docker --stack python --dry-run
devherd sentry init hello-vue-flask-docker --stack node --dry-run
devherd down /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Si Caddy esta instalado y configurado en el host, agrega:

```bash
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker
```

## 7. Flujo objetivo con `local_proxy`

Este flujo ya esta integrado en el codigo actual cuando el driver es `caddy-docker-external`.

Objetivo:

- no instalar Caddy en el host
- reutilizar `/home/elyarestark/infra/local_proxy`
- usar dominios `*.localhost`
- conectar los proyectos Docker a la red `infra_web`

### Resultado deseado

El flujo final deberia verse asi:

```bash
devherd init --proxy caddy-docker-external
devherd doctor
devherd park /home/elyarestark/develop/examples
devherd plan /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker
```

Y el dominio esperado deberia ser:

```text
http://mi-demo.localhost
```

### Que hace DevHerd en ese modo

- detectar que el driver de proxy es `caddy-docker-external`
- generar un override de Compose administrado por DevHerd
- conectar `frontend` y `backend` a `infra_web`
- asignar aliases estables como `mi-demo-frontend` y `mi-demo-backend`
- escribir un bloque administrado dentro de `/home/elyarestark/infra/local_proxy/Caddyfile`
- recargar el contenedor `caddy`
- abrir `http://mi-demo.localhost`

Nombre actual del override:

```text
.devherd.proxy.override.yml
```

### Regla de proxy esperada para este proyecto

La ruta ideal para `Vue + Flask` es:

- `/` hacia el frontend
- `/api/*` hacia el backend

Ejemplo conceptual:

```caddy
http://mi-demo.localhost {
    handle /api/* {
        reverse_proxy mi-demo-backend:8000
    }

    handle {
        reverse_proxy mi-demo-frontend:5173
    }
}
```

### Compatibilidad esperada del proyecto Docker

Para que esa integracion funcione bien, el proyecto deberia poder:

- levantarse con `docker compose`
- exponer frontend y backend como servicios separados
- aceptar una red Docker externa compartida
- tolerar un archivo Compose override generado por DevHerd
- o bien declarar `proxy.service` y `proxy.port` dentro de `.devherd.yml`

### Aislamiento recomendado para multiples proyectos

Cuando un proyecto define `container_name`, debe parametrizarlo para permitir clones o variantes paralelas:

```yaml
services:
  app:
    container_name: ${COMPOSE_NAME_PREFIX:-aang}_app
  web:
    container_name: ${COMPOSE_NAME_PREFIX:-aang}_web
```

En `.env`, cada instancia local debe tener identidad propia:

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

Para una segunda copia del mismo proyecto:

```env
COMPOSE_NAME_PREFIX=aang-v2
APP_URL=http://aang-v2.localhost
SESSION_COOKIE=aang_v2_session
CACHE_PREFIX=aang_v2_cache_
REDIS_PREFIX=aang_v2_database_
APP_PORT=8084
FORWARD_DB_PORT=3311
```

Este patron ya se aplico y valido en `aang-server` y `Uniformes`: ambos pueden quedar arriba con dominios `.localhost`, cookies separadas y puertos propios.

### Project-name estable por ruta

DevHerd ejecuta Compose con un project-name estable por ruta:

```text
devherd-<slug-del-proyecto>-<hash-de-ruta>
```

Ejemplos reales:

```text
devherd-aang-server-7b54ffae
devherd-uniformes-e2102970
```

Esto evita que dos carpetas con el mismo basename compartan el project-name de Compose. Tambien hace que `plan`, `up`, `down`, `stop`, `inspect` y proxy usen la misma identidad.

Cuando se cambia el project-name, Compose tambien cambia los nombres default de los volumenes internos. Para preservar datos, los volumenes importantes deben tener `name:` parametrizado:

```yaml
volumes:
  db_data:
    name: ${DB_VOLUME_NAME:-mi_proyecto_db_data}
    external: ${DB_VOLUME_EXTERNAL:-false}
```

En `.env`:

```env
DB_VOLUME_NAME=mi_proyecto_db_data
DB_VOLUME_EXTERNAL=false
```

En `aang-server`, durante la migracion se fijo `DB_VOLUME_NAME=aang-server_aang_db_data` y `DB_VOLUME_EXTERNAL=true` para seguir usando el volumen MySQL legado. `Uniformes` ya usaba un volumen externo explicito (`uniformes_db_data`).

## 8. Flujo completo para proyectos reales

Este es el flujo recomendado para trabajar con proyectos reales ya integrados con `.devherd.yml`, `caddy-docker-external` y dominios `.localhost`.

### Primera vez en una maquina

Inicializa DevHerd con el proxy Docker externo:

```bash
devherd init --proxy caddy-docker-external
devherd doctor
```

Registra la carpeta donde viven los proyectos:

```bash
devherd park /home/elyares/develop/work
devherd list
```

Antes de levantar un proyecto, revisa su stack y posibles choques:

```bash
devherd plan /home/elyares/develop/work/aang-server
devherd inspect /home/elyares/develop/work/aang-server
```

Levanta el proyecto, aplica proxy y abre el dominio:

```bash
devherd up /home/elyares/develop/work/aang-server
devherd proxy apply aang-server
devherd open aang-server
```

`devherd up` ejecuta preflight automaticamente. Si hay warnings, los imprime y continua. Si hay fallos, se detiene antes de levantar contenedores.

Valida por HTTP:

```bash
curl -I http://aang.localhost
```

Para `Uniformes`, el flujo equivalente es:

```bash
devherd plan /home/elyares/develop/work/Uniformes
devherd inspect /home/elyares/develop/work/Uniformes
devherd up /home/elyares/develop/work/Uniformes
devherd proxy apply Uniformes
devherd open Uniformes
curl -I http://uniformes.localhost
```

### Ciclo diario si el proyecto ya esta levantado

Primero revisa que esta registrado y que el proxy sigue sano:

```bash
devherd list
devherd inspect /home/elyares/develop/work/aang-server
```

Si el proyecto ya esta arriba y solo quieres seguir trabajando, no necesitas volver a correr `up`. Abre el dominio:

```bash
devherd open aang-server
```

Si cambiaste `.env`, `docker-compose.yml`, dependencias de imagen o necesitas recrear contenedores, vuelve a ejecutar:

```bash
devherd up /home/elyares/develop/work/aang-server
devherd proxy apply aang-server
```

`up` es idempotente: Compose reutiliza lo que ya esta arriba y recrea solo lo que necesite segun cambios.

Si necesitas saltarte el preflight por una razon puntual:

```bash
devherd up --no-inspect /home/elyares/develop/work/aang-server
```

Si el preflight marca `FAIL` pero sabes que quieres continuar:

```bash
devherd up --force /home/elyares/develop/work/aang-server
```

### Bajar limpio para que no quede todo arriba

Cuando termines de trabajar en un proyecto, bajalo con DevHerd:

```bash
devherd down /home/elyares/develop/work/aang-server
```

En modo `caddy-docker-external`, esto hace tres cosas importantes:

- ejecuta `docker compose down` con los mismos compose files del proyecto
- elimina `.devherd.proxy.override.yml`
- remueve el bloque del dominio del `local_proxy/Caddyfile`

Despues puedes validar que ya no quedo publicado:

```bash
devherd inspect /home/elyares/develop/work/aang-server
```

Para levantarlo de nuevo:

```bash
devherd up /home/elyares/develop/work/aang-server
devherd proxy apply aang-server
devherd inspect /home/elyares/develop/work/aang-server
devherd open aang-server
```

### Bajar varios proyectos

Si tienes ambos arriba y quieres dejar limpio el entorno:

```bash
devherd down /home/elyares/develop/work/aang-server
devherd down /home/elyares/develop/work/Uniformes
```

Los servicios compartidos administrados por DevHerd, como `infra_redis`, viven aparte. Si tambien quieres apagarlos:

```bash
devherd service stop redis
devherd service stop mailpit
```

No uses `docker compose down` manualmente salvo que estes depurando algo puntual: si lo haces, puedes dejar bloques stale en Caddy o archivos override generados por DevHerd. El camino normal es `devherd down`.

## 9. Recomendacion practica

Mientras validas stacks reales, usa este criterio:

- para validar DevHerd hoy: `init`, `doctor`, `park`, `list`, `plan`, `domain set`, `up`, `proxy apply`, `open`, `down`, `sentry --dry-run`
- para auditar manualmente: `inspect`
- para validar la app real: entra por `127.0.0.1:5173` y `127.0.0.1:8000`
- para publicar dominios `.test`: usa Caddy local en host
- para tu entorno definitivo: usa `caddy-docker-external` con `.localhost`

## 10. Siguiente iteracion recomendada

El siguiente bloque de trabajo con mas valor para tu caso es:

1. extender la validacion a `poderygozo-landing-page` y `RetailDataOps`
2. reducir la dependencia de `container_name` en proyectos reales
3. ampliar el contrato de proxy para mas frameworks o perfiles
4. validar clones paralelos del mismo proyecto
