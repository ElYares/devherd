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

Si estas fuera de la carpeta del proyecto:

```bash
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
```

Si ya estas dentro del proyecto:

```bash
devherd up
```

Esto ejecuta `docker compose up` sobre ese proyecto.

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

Esta es la direccion recomendada para tu entorno, pero todavia no esta integrada en el codigo actual.

Objetivo:

- no instalar Caddy en el host
- reutilizar `/home/elyarestark/infra/local_proxy`
- usar dominios `*.localhost`
- conectar los proyectos Docker a la red `infra_web`

### Resultado deseado

El flujo final deberia verse asi:

```bash
devherd init
devherd doctor
devherd park /home/elyarestark/develop/examples
devherd domain set hello-vue-flask-docker --domain mi-demo
devherd up /home/elyarestark/develop/examples/hello-vue-flask-docker
devherd proxy apply hello-vue-flask-docker
devherd open hello-vue-flask-docker
```

Y el dominio esperado deberia ser:

```text
http://mi-demo.localhost
```

### Que tendria que hacer DevHerd en ese modo

- detectar que el driver de proxy es `caddy-docker-external`
- generar un override de Compose administrado por DevHerd
- conectar `frontend` y `backend` a `infra_web`
- asignar aliases estables como `mi-demo-frontend` y `mi-demo-backend`
- escribir un bloque administrado dentro de `/home/elyarestark/infra/local_proxy/Caddyfile`
- recargar el contenedor `caddy`
- abrir `http://mi-demo.localhost`

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

## 8. Recomendacion practica

Mientras no se cierre la integracion con `local_proxy`, usa este criterio:

- para validar DevHerd hoy: `init`, `doctor`, `park`, `list`, `domain set`, `up`, `down`, `sentry --dry-run`
- para validar la app real: entra por `127.0.0.1:5173` y `127.0.0.1:8000`
- para publicar dominios `.test` hoy: usa Caddy local en host
- para tu entorno definitivo: migrar DevHerd al modo `local_proxy` con `.localhost`

## 9. Siguiente iteracion recomendada

El siguiente bloque de trabajo con mas valor para tu caso es:

1. integrar `local_proxy` como driver externo de Caddy
2. soportar `.localhost` como TLD por defecto en ese modo
3. generar un Compose override administrado por DevHerd
4. recargar Caddy dentro del contenedor, no en el host
