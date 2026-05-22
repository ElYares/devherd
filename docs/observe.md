# DevHerd Observe

DevHerd Observe es el modulo local de observabilidad de DevHerd. Su objetivo es registrar fallas de proyectos locales, agruparlas como issues, correlacionarlas con contenedores y logs, y mostrarlas en un panel local sin depender de Sentry Cloud ni de un Sentry self-hosted completo.

No busca reemplazar Sentry en produccion. Busca dar una experiencia local integrada a DevHerd para entender como fallo un proyecto durante desarrollo.

## 1. Objetivo

Flujo conceptual:

```text
SDK / logs / Docker events
   ↓
Normalizacion del error
   ↓
Agrupacion en issues
   ↓
Base de datos local
   ↓
Panel para ver errores
   ↓
Alertas locales
```

DevHerd Observe debe responder estas preguntas:

- que proyecto fallo
- que servicio o contenedor genero la falla
- cual fue la excepcion o mensaje principal
- donde ocurrio la falla, incluyendo archivo, funcion y linea cuando exista
- que paso antes de fallar, usando breadcrumbs, request metadata y logs cercanos
- cuantas veces se repitio el mismo problema
- si el problema sigue activo, fue visto, resuelto o ignorado

## 2. Principio de aislamiento

Observe debe vivir dentro de DevHerd y no debe afectar produccion.

Reglas:

- No se hardcodea ningun DSN en codigo fuente del proyecto.
- No se escriben secretos en el repositorio del proyecto.
- La integracion se activa solo por archivos locales administrados por DevHerd, como `.devherd.observe.override.yml` o `.env.devherd`.
- Si Observe no esta levantado, el proyecto debe poder seguir arrancando sin fallar.
- El patron recomendado en codigo es inicializar SDKs solo cuando `SENTRY_DSN` existe.
- El entorno debe marcarse como `local` o `devherd-local`, nunca como `production`.
- Los datos se guardan en una base local separada de la base principal de DevHerd.

Ruta propuesta de la base:

```text
~/.local/share/devherd/observability/devherd-observe.db
```

## 3. Componentes

### Collector local

Proceso local iniciado por:

```bash
devherd observe start
```

Responsabilidades:

- escuchar en `127.0.0.1:9777` por defecto
- aceptar eventos JSON simples para pruebas y tooling propio
- aceptar un primer corte de envelopes compatibles con SDKs Sentry
- normalizar eventos
- agrupar eventos en issues
- persistir eventos, issues y payload crudo

### Base de datos

Base SQLite separada para observabilidad.

Tablas actuales:

- `issues`: problemas agrupados por fingerprint
- `events`: ocurrencias individuales
- `containers`: metadata de contenedores observados
- `container_events`: cambios de status y restart count
- `container_logs`: logs cercanos a una falla
- `alerts`: reglas de alerta local
- `alert_deliveries`: historial de alertas enviadas

Tablas pendientes para una version posterior:

- `stack_frames`: frames normalizados por evento
- `breadcrumbs`: trayectoria previa al error

### CLI

Comandos principales:

```bash
devherd observe start
devherd observe status
devherd observe open
devherd observe dsn <project>
devherd observe issues [project]
devherd observe events [project]
devherd observe attach <project> --stack laravel --dry-run
devherd observe attach <project> --stack laravel
devherd observe detach <project>
devherd observe scan [project]
devherd observe containers [project]
devherd observe timeline <event-id>
devherd observe cleanup --days 14
devherd observe alert add --project aang-server --on new-issue
devherd observe alert add --project aang-server --on error-rate --threshold 10 --window 5m
devherd observe alert add --project aang-server --on container-exit
devherd observe alert list [project]
devherd observe alert deliveries [project]
devherd observe alert remove <id>
```

## 4. Ingestion

### Endpoint simple de DevHerd

Para pruebas locales y herramientas propias:

```text
POST http://127.0.0.1:9777/api/<project>/event
```

Payload ejemplo:

```json
{
  "message": "Undefined variable $user",
  "exception_type": "ErrorException",
  "level": "error",
  "platform": "php",
  "service": "app",
  "container": "aang_app",
  "culprit": "app/Http/Controllers/UserController.php:42",
  "environment": "local"
}
```

### Endpoint tipo Sentry envelope

Primer corte compatible:

```text
POST http://127.0.0.1:9777/api/<project>/envelope/
```

DSN local esperado:

```text
http://devherd@127.0.0.1:9777/<project>
```

El collector no valida autenticacion actualmente porque solo escucha en loopback.

## 5. Normalizacion

Cada evento se transforma a un formato comun:

- `project`
- `event_id`
- `timestamp`
- `level`
- `platform`
- `service`
- `container`
- `exception_type`
- `message`
- `culprit`
- `transaction`
- `environment`
- `release`
- `raw_payload`

Campos derivados:

- `fingerprint`
- `title`
- `first_seen`
- `last_seen`
- `event_count`

## 6. Agrupacion de issues

Fingerprint inicial:

```text
exception_type + normalized_message + culprit + service
```

Reglas:

- eventos con el mismo fingerprint actualizan el mismo issue
- `first_seen` se conserva
- `last_seen` se actualiza
- `event_count` incrementa
- estado default: `new`

Estados:

- `new`
- `seen`
- `resolved`
- `ignored`

## 7. Correlacion con contenedores

Fase 2 agrega labels Docker en overrides administrados por DevHerd:

```yaml
labels:
  devherd.project: aang-server
  devherd.service: web
  devherd.observe: "true"
```

Con eso Observe podra vincular:

- proyecto
- servicio Compose
- contenedor real
- imagen
- estado del contenedor
- logs alrededor de la falla

## 8. Trayectoria de la falla

Primer corte:

```text
request / job / command
   ↓
breadcrumbs del SDK
   ↓
excepcion
   ↓
stack trace
   ↓
logs del contenedor alrededor del timestamp
   ↓
estado final del contenedor
```

Ejemplo de salida esperada:

```text
12:03:10 request GET /checkout
12:03:11 breadcrumb db.query users
12:03:12 breadcrumb redis cache miss
12:03:13 exception PaymentGatewayTimeout
12:03:13 container log upstream timeout
12:03:14 container stayed running
```

El corte actual guarda el evento normalizado, el payload crudo, contenedores observados, eventos de contenedor y logs cercanos cuando Docker puede entregarlos. Breadcrumbs y stack frames quedan pendientes para una version posterior.

## 9. Fases de implementacion

### Fase 1: Collector y persistencia local

- crear `internal/observe`
- crear schema SQLite separado
- implementar `devherd observe start`
- implementar healthcheck con `devherd observe status`
- implementar endpoint `POST /api/<project>/event`
- implementar endpoint `POST /api/<project>/envelope/`
- implementar normalizacion basica
- implementar agrupacion basica en issues
- implementar `devherd observe issues [project]`
- implementar `devherd observe events [project]`

### Fase 2: Attach por proyecto

- implementar `devherd observe attach`: hecho
- generar `.devherd.observe.override.yml`: hecho
- inyectar variables locales como `SENTRY_DSN`, `SENTRY_ENVIRONMENT` y `DEVHERD_OBSERVE`: hecho
- agregar labels Docker por proyecto y servicio: hecho
- asegurar que `up`, `stop` y `down` incluyan el override cuando exista: hecho
- implementar `detach`: hecho

### Fase 3: Correlacion con Docker

- leer metadata de contenedores con labels `devherd.*`: hecho
- capturar logs cercanos al timestamp del evento: hecho
- detectar cambios de status y restart count: hecho
- relacionar eventos con contenedor y servicio Compose: hecho
- listar contenedores observados con `devherd observe containers`: hecho
- mostrar trayectoria con `devherd observe timeline <event-id>`: hecho

Comandos de fase 3:

```bash
devherd observe scan [project]
devherd observe containers [project]
devherd observe timeline <event-id>
```

### Fase 4: Panel local

- implementar `devherd observe open`: hecho
- crear panel web local en `http://127.0.0.1:9777/observe`: hecho
- listar issues agrupados: hecho
- mostrar eventos recientes: hecho
- mostrar contenedores observados: hecho
- mostrar alertas locales disparadas: hecho
- mostrar timeline por evento con logs cercanos: hecho
- mostrar stack trace y breadcrumbs normalizados: pendiente

### Fase 5: Alertas

- alertas por nuevo issue: hecho
- alertas por error rate: hecho
- alertas por contenedor caido: hecho
- alertas por restart de contenedor: hecho
- historial local en `alert_deliveries`: hecho
- salida CLI para automatizacion: hecho
- notificaciones del sistema operativo: pendiente

## 10. Flujo de uso en un proyecto

### 1. Arrancar el collector local

En una terminal dedicada:

```bash
devherd observe start
```

Esto levanta el collector en `127.0.0.1:9777`, crea o migra la base SQLite local y empieza a aceptar eventos.

Puedes validar el proceso con:

```bash
devherd observe status
```

Y abrir el panel local con:

```bash
devherd observe open
```

### 2. Revisar el DSN local del proyecto

```bash
devherd observe dsn aang-server
```

Salida esperada:

```text
http://devherd@127.0.0.1:9777/aang-server
```

Ese DSN solo apunta al collector local de DevHerd.

### 3. Generar el override Compose local

Primero revisa lo que se va a escribir:

```bash
devherd observe attach aang-server --stack laravel --dry-run
```

Luego aplica el override:

```bash
devherd observe attach aang-server --stack laravel
```

Tambien puedes limitarlo a un servicio:

```bash
devherd observe attach aang-server --stack laravel --service web
```

Esto crea `.devherd.observe.override.yml` en la raiz del proyecto. El archivo inyecta `SENTRY_DSN`, `SENTRY_ENVIRONMENT`, `DEVHERD_OBSERVE`, `DEVHERD_PROJECT` y labels `devherd.*`.

### 4. Levantar el proyecto

```bash
devherd up aang-server
```

Si `.devherd.observe.override.yml` existe, `devherd up`, `devherd stop` y `devherd down` lo incluyen automaticamente. Si Observe no esta levantado, el proyecto debe seguir arrancando; los SDKs solo deben inicializarse cuando `SENTRY_DSN` existe y no deben romper el boot por no poder enviar eventos.

### 5. Capturar una falla

Puedes provocar una falla real del proyecto o enviar un evento manual:

```bash
curl -X POST http://127.0.0.1:9777/api/aang-server/event \
  -H 'Content-Type: application/json' \
  -d '{"message":"demo failure","exception_type":"DemoError","service":"web"}'
```

### 6. Inspeccionar desde CLI

```bash
devherd observe issues aang-server
devherd observe events aang-server
devherd observe timeline <event-id>
```

`issues` muestra problemas agrupados, `events` muestra ocurrencias recientes y `timeline` muestra evento, contenedor, eventos del contenedor y logs cercanos.

### 7. Correlacionar contenedores

El collector escanea periodicamente contenedores observados mientras esta corriendo. Tambien puedes forzar un scan:

```bash
devherd observe scan aang-server
devherd observe containers aang-server
```

`scan` lee Docker por labels `devherd.observe=true` y guarda snapshots. `containers` lista lo que Observe conoce del proyecto.

### 8. Crear alertas locales

```bash
devherd observe alert add --project aang-server --on new-issue
devherd observe alert add --project aang-server --on error-rate --threshold 10 --window 5m
devherd observe alert add --project aang-server --on container-exit
devherd observe alert add --project aang-server --on container-restart
```

Las alertas no salen a servicios externos. Se guardan como entregas locales y se ven desde CLI o desde el panel:

```bash
devherd observe alert list aang-server
devherd observe alert deliveries aang-server
```

### 9. Limpiar datos viejos

```bash
devherd observe cleanup --days 14
```

Elimina eventos, logs, eventos de contenedor, entregas de alerta e issues con datos anteriores al corte indicado.

### 10. Desactivar Observe en el proyecto

```bash
devherd observe detach aang-server
```

Elimina `.devherd.observe.override.yml`. No toca codigo fuente ni configuracion de produccion.

## 11. Comandos y como funcionan

- `devherd observe start`: arranca el collector HTTP y el panel local en foreground.
- `devherd observe status`: consulta `GET /health` del collector.
- `devherd observe open`: abre `http://127.0.0.1:9777/observe` en el navegador o imprime la URL si no puede abrirlo.
- `devherd observe dsn <project>`: imprime el DSN local para SDKs tipo Sentry.
- `devherd observe attach <project-or-path> --stack <stack>`: genera el override Compose local de observabilidad.
- `devherd observe detach <project-or-path>`: elimina el override local.
- `devherd observe scan [project]`: inspecciona contenedores Docker etiquetados y guarda snapshots.
- `devherd observe containers [project]`: lista contenedores observados desde la base local.
- `devherd observe issues [project]`: lista issues agrupados por fingerprint.
- `devherd observe events [project]`: lista eventos recientes y sus ids.
- `devherd observe timeline <event-id>`: muestra el flujo local de la falla.
- `devherd observe alert add`: crea una regla de alerta local.
- `devherd observe alert list`: lista reglas de alerta.
- `devherd observe alert deliveries`: lista alertas disparadas.
- `devherd observe alert remove <id>`: elimina una regla.
- `devherd observe cleanup --days <n>`: borra datos locales viejos.

## 12. Criterio de exito del MVP

El MVP inicial se considera util cuando este flujo funciona:

```bash
devherd observe start
devherd observe dsn aang-server
devherd observe open
curl -X POST http://127.0.0.1:9777/api/aang-server/event \
  -H 'Content-Type: application/json' \
  -d '{"message":"demo failure","exception_type":"DemoError","service":"web"}'
devherd observe issues aang-server
devherd observe events aang-server
```

Y DevHerd muestra:

- un issue agrupado
- un evento asociado al proyecto
- titulo y mensaje normalizados
- servicio/contenedor cuando existan
- contador de ocurrencias
