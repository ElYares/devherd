# DevHerd Observe

DevHerd Observe es el modulo local de observabilidad de DevHerd. Su objetivo es registrar fallas de proyectos locales, agruparlas como issues, correlacionarlas con contenedores y logs, y mostrarlas en un panel local sin depender de Sentry Cloud ni de un Sentry self-hosted completo.

No busca reemplazar Sentry en produccion. Busca dar una experiencia local integrada a DevHerd para entender como fallo un proyecto durante desarrollo.

> **Para integrarlo en un proyecto concreto** ver
> [guides/observe-laravel.md](guides/observe-laravel.md): reporter listo para copiar,
> soporte de contexto y los **requisitos de red** de la ingesta desde contenedores.

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
- La integracion se activa solo por un archivo local administrado por DevHerd: `.devherd.observe.override.yml`.
- Si Observe no esta levantado, el proyecto debe poder seguir arrancando sin fallar.
- El patron recomendado en codigo es inicializar SDKs solo cuando `SENTRY_DSN` existe.
- El entorno debe marcarse como `local` o `devherd-local`, nunca como `production`.
- Los datos se guardan en una base local separada de la base principal de DevHerd.

Ruta de la base (Linux):

```text
~/.local/share/devherd/observability/devherd-observe.db
```

En macOS y Windows el directorio de datos cae bajo `os.UserConfigDir()`, salvo que definas
`XDG_DATA_HOME`.

> Nota de mantenimiento: a diferencia de la base principal, la base de Observe **no tiene
> migraciones versionadas**. El esquema completo se reejecuta en cada invocacion y solo es
> idempotente porque todo es `CREATE ... IF NOT EXISTS`; cualquier cambio de columna futuro
> requerira trabajo manual.

### Alcanzabilidad desde contenedores

Dentro de un contenedor, `127.0.0.1` es el propio contenedor, no el host. Por eso el
collector y el DSN **no pueden vivir solo en loopback** si el proyecto observado esta
dockerizado, que es el caso de uso principal.

DevHerd resuelve esto solo. `observe start` escucha **a la vez** en `127.0.0.1:9777` y en el
gateway de cada red relevante: las administradas por DevHerd (`infra_web` del proxy e
`infra_net` de los servicios compartidos) y las de los contenedores ya observados.

```bash
devherd observe start
# observe collector: http://127.0.0.1:9777
# observe collector: http://172.18.0.1:9777
# observe collector: http://172.20.0.1:9777
# containers on infra_web, infra_net should use http://172.20.0.1:9777
```

**Por que varias redes y no solo la del proxy.** A `infra_web` se conecta unicamente el
servicio que publica el proxy —el nginx, el front—, no el que reporta. Medido sobre dos
proyectos reales:

| Proyecto | Red propia | `infra_net` | `infra_web` |
|---|---|---|---|
| aang-server | 6/6 contenedores | 2/6 (`app`, `queue`) | 1/6 (`web`) |
| tl-mas-server | 4/4 contenedores | — | 1/4 (`app`) |

Por eso `attach` y `dsn` no asumen una red: cuentan cuantos contenedores del proyecto hay en
cada una y eligen **la red administrada por DevHerd con mayor cobertura**. Se prefiere una
red estable aunque cubra menos contenedores, porque la red privada del proyecto cambia de
subred cada vez que `compose` la recrea y dejaria el DSN inyectado apuntando a una direccion
que ya no existe. Solo se cae a la red privada cuando ninguna red DevHerd toca el proyecto.

Asi el panel te sigue sirviendo en `http://127.0.0.1:9777/observe` y los contenedores tienen
una IP alcanzable. Al ser subredes privadas de Docker **no son ruteables desde la LAN** (a
diferencia de `--addr 0.0.0.0`, que si te expondria).

Si pasas `--addr` explicito mandas tu: DevHerd no añade gateways, y avisa por stderr si la
direccion resultante es loopback.

**Con `ufw` activo hace falta una regla explicita.** El trafico contenedor -> host se
descarta en `INPUT`. Los puertos publicados por Docker funcionan porque sus reglas DNAT
preceden a las cadenas de ufw, pero el collector es un listener normal del host:

```bash
sudo ufw allow from 172.18.0.0/16 to 172.18.0.1 port 9777 proto tcp \
  comment 'devherd observe collector'
```

Cada red necesita **su propia regla**, porque cambian tanto la subred de origen como el
gateway de destino. `devherd observe firewall` las deriva todas y `--apply` las aplica con
aviso previo de `sudo`, igual que la sincronizacion de `/etc/hosts`:

```bash
devherd observe firewall
devherd observe firewall --apply
```

`devherd observe status [proyecto]` verifica el resultado **desde dentro de un contenedor**,
que es donde el fallo se manifiesta: el host siempre se alcanza a si mismo aunque el
cortafuegos bloquee el trafico de Docker. Con un proyecto, la sonda corre en la red de **ese
proyecto**; sin el, en una red compartida.

```bash
devherd observe status aang-server
# container reachability (aang-server on infra_net): ok at http://172.20.0.1:9777
```

Pasarle el proyecto importa: una sonda lanzada en la red equivocada devuelve un `ok` que no
significa nada para el proyecto que te interesa.

Cuando falla, imprime la regla concreta que falta (y detecta si ufw esta activo leyendo
`/etc/ufw/ufw.conf`, sin pedir root). La sonda usa la primera imagen que ya tengas en local
—`busybox`, `alpine` o `caddy:2-alpine`— y **nunca descarga ninguna**: si no hay ninguna,
imprime el comando equivalente para ejecutarlo a mano. Se desactiva con
`--check-reachability=false`.

### El collector como servicio de usuario

El collector es un proceso en foreground: si cierras la terminal deja de ingerir y los
errores se pierden sin rastro. Para que sobreviva y se reinicie solo:

```bash
devherd observe daemon install     # unidad systemd --user, habilitada y arrancada
devherd observe daemon status
devherd observe daemon uninstall
```

La unidad apunta al binario que ejecuta el comando (`os.Executable()`), asi que si
reinstalas DevHerd en otra ruta hay que volver a instalarla.

> Ver [guides/observe-laravel.md](guides/observe-laravel.md) para el procedimiento completo
> en un proyecto Laravel.

## 3. Componentes

### Collector local

Proceso local iniciado por:

```bash
devherd observe start
```

Responsabilidades:

- escuchar en `127.0.0.1:9777` y en el gateway de la red compartida por defecto
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

Tambien se acepta la ruta legacy `POST /api/<project>/store`, equivalente al endpoint
simple. El cuerpo esta limitado a 2 MiB y la respuesta es `202 Accepted`.

> **Limitaciones actuales del parser de envelopes.** No descomprime `gzip`, que es el
> default de varios SDKs oficiales (Python, PHP, Node); ignora el campo `length` del
> header de item; asume una sola linea por payload; y descarta el array `fingerprint` que
> envian los SDKs. En la practica, un SDK Sentry oficial puede no funcionar contra el
> collector sin desactivar la compresion.

> **Seguridad.** El collector **no valida autenticacion**: ignora `X-Sentry-Auth` y el
> parametro `sentry_key`. Es aceptable porque escucha en loopback por defecto, pero si
> cambias `--addr` a `0.0.0.0` expones la ingesta y el panel a toda la red sin ninguna
> barrera.

> **Nombre reservado.** Un proyecto llamado `observe` colisiona con la ruta del panel
> (`/api/observe/`) y su ingesta devuelve 405. Usa otro nombre.

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

### Datos fuera del modelo normalizado

`raw_payload` guarda el payload original completo, tal cual llego. Todo lo que un emisor
mande y no encaje en las columnas de arriba —`context`, `tags`, breadcrumbs, stack frames—
sobrevive ahi.

Para consultarlo no hace falta abrir SQLite: `observe timeline` imprime un bloque
`Payload:` y la API del panel devuelve `payload_extra`, ambos ya filtrados para omitir las
claves que duplican una columna existente (helper `observe.ExtraPayload`).

```
Exception: TimbradoFallidoException
Message: El SAT rechazo el timbrado

Payload:
- context: {"cfdi_uuid":"A1B2-C3D4","intento":3,"reintentable":true}
```

Un emisor que quiera adjuntar contexto propio solo tiene que incluirlo en el JSON:

```json
{ "message": "...", "context": { "factura_id": 9182 } }
```

> Historico: hasta la exposicion de `raw_payload`, ninguna consulta leia esa columna, de
> modo que el contexto se escribia y quedaba inaccesible salvo por SQL directo. Si tu
> binario no imprime el bloque `Payload:`, es anterior a ese cambio.

## 6. Agrupacion de issues

Hay dos formas de agrupar: la derivada del evento y la que fija el cliente.

### Fingerprint derivado

Es un **SHA-1** de cinco componentes unidos por salto de linea:

```text
project + exception_type + normalized_message + culprit + service
```

`normalized_message` aplica trim, minusculas, colapso de espacios y **enmascara lo que
cambia en cada ocurrencia**, en este orden:

| Patron | Se sustituye por | Ejemplo |
|---|---|---|
| Correos | `<email>` | `no account for ana@x.mx` → `no account for <email>` |
| UUIDs | `<uuid>` | `order 550e8400-...-446655440000` → `order <uuid>` |
| Hexadecimales de 12+ | `<hash>` | `token 9f86d081884c` → `token <hash>` |
| Numeros sueltos | `<n>` | `user 42 not found` → `user <n> not found` |

Asi `"user 42 not found"` y `"user 43 not found"` comparten issue, que era el principal
limitante practico del agrupamiento.

> **Contrapartida asumida**: mensajes que solo difieren en un numero se agrupan juntos, asi
> que `"http 404"` y `"http 500"` caen en el mismo issue. Cuando esa distincion importa,
> separalas por `exception_type` o manda un fingerprint explicito.

### Fingerprint explicito

El evento puede traer un campo `fingerprint` y entonces manda el cliente: mensaje y culprit
dejan de influir en la agrupacion.

```json
{"exception_type": "LoginLocked", "message": "Blocked after 5 tries", "fingerprint": "login-lockout"}
```

Se acepta como string o como lista (que es el formato de los SDK tipo Sentry, y se une con
`|`). La clave se mezcla con el proyecto antes de hashear, asi que dos proyectos con el mismo
`fingerprint` **no** comparten issue.

Otro detalle a tener en cuenta: el fingerprint se calcula **antes** de enriquecer el evento
con los datos del contenedor, asi que el `service` inferido desde Docker no participa en la
agrupacion. Dos eventos equivalentes pueden separarse segun si el SDK envio `service`
explicitamente o no.

Reglas:

- eventos con el mismo fingerprint actualizan el mismo issue
- `first_seen` se conserva
- `last_seen` se actualiza
- `event_count` incrementa
- estado default: `new`
- un issue en estado `resolved` que vuelve a recibir eventos regresa a `new`

Estados previstos en el esquema: `new`, `seen`, `resolved`, `ignored`.

> **Estado real**: hoy **no existe ningun comando ni endpoint para cambiar el estado de un
> issue**, de modo que en la practica todos quedan en `new`. La transicion automatica
> `resolved → new` esta implementada, pero nada puede marcar un issue como resuelto todavia.

## 7. Correlacion con contenedores

**Implementado.** El override administrado por DevHerd anade estas labels a cada servicio
observado:

```yaml
labels:
  devherd.project: aang-server
  devherd.service: web
  devherd.observe: "true"
```

Con eso Observe vincula:

- proyecto
- servicio Compose
- contenedor real
- imagen
- estado del contenedor
- logs alrededor de la falla

El collector consulta Docker en cada ingesta y ademas mantiene un poller que toma
instantaneas cada 10 segundos. La seleccion del contenedor es por cascada: primero por
nombre o ID de contenedor declarado en el evento, luego por servicio, y si el proyecto
tiene un unico contenedor observado, ese. **No se usa ningun criterio temporal** para
elegir contenedor; la ventana de tiempo (±30 s por defecto) solo decide que logs se traen.

### Cadencias: cada cosa se registra en un momento distinto

| Que | Cuando | Consecuencia practica |
|---|---|---|
| **Errores** | Al instante, push del emisor | Sin cola ni reintento: **si el collector no corre, el error se pierde** |
| **Logs del contenedor** | Una sola vez, al ingerir el evento | `docker logs --since t-30s --until t+30s --tail 200`. No se rellenan despues |
| **Estado de contenedores** | Cada 10 s (poller), o manual con `observe scan` | Un contenedor que cae y vuelve dentro del intervalo puede pasar inadvertido |
| **Alertas** | Se evaluan al ingerir cada evento | Ver los avisos de la seccion de alertas |

Dos corolarios que explican comportamientos que parecen bugs:

- **`Container logs: none captured` suele ser correcto.** Si la app estaba ociosa no habia
  lineas en la ventana, y como la captura es de una sola pasada, nunca se rellenaran.
- **La mitad futura de la ventana casi siempre esta vacia**, porque se consulta en el
  instante del evento y esos logs todavia no existen.

**Observe no es un agregador de logs**: no sigue los contenedores de forma continua ni
almacena su salida. Solo toma una foto de 60 segundos alrededor de cada error. Para ver
logs completos sigue estando `devherd logs` o `docker compose logs`.

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

Esto crea `.devherd.observe.override.yml` en la raiz del proyecto. El archivo inyecta
`SENTRY_DSN`, `SENTRY_ENVIRONMENT`, `DEVHERD_OBSERVE`, `DEVHERD_PROJECT`,
`DEVHERD_OBSERVE_STACK` y las labels `devherd.observe`, `devherd.project`,
`devherd.service` y `devherd.stack`.

> `attach` **reescribe el archivo completo**, no lo fusiona con lo anterior. Si ejecutas
> `attach --service api` y despues `attach --service web`, solo queda `web`. Para observar
> varios servicios, pasalos en la misma invocacion:
> `--service api,web`.

> El `--stack` solo cambia el valor de `DEVHERD_OBSERVE_STACK` y de la label
> `devherd.stack`. Hoy **no genera configuracion especifica por stack** (ni variables
> `NEXT_PUBLIC_*`, ni sample rates, ni instalacion de SDKs).

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

Las alertas no salen a servicios externos. Una "entrega" es unicamente una fila en la base
local: no hay webhooks, ni ejecucion de comandos, ni notificaciones del sistema, ni correo.
Se consultan desde la CLI o desde el panel:

```bash
devherd observe alert list aang-server
devherd observe alert deliveries aang-server
```

Se evaluan de forma sincrona al ingerir cada evento o instantanea de contenedor; no hay
motor de alertas ni planificador.

> `error-rate` **no tiene periodo de enfriamiento ni deduplicacion**: una vez superado el
> umbral, cada evento posterior dentro de la ventana genera otra entrega. Medido: con
> umbral 3 y ventana 5m, 4 eventos produjeron 2 entregas de `error-rate` (la 3.a y la 4.a).

> `new-issue` dispara **por cada issue nuevo**, no una vez por proyecto. Combinado con que
> el fingerprint no enmascara numeros ni identificadores, un mismo bug con mensajes
> variables (`user 42`, `user 43`, ...) genera un issue y por tanto una alerta por cada
> variante. Medido: 4 errores distintos = 4 entregas de `new-issue`.

> No hay forma de deshabilitar una regla sin borrarla: `enabled` siempre se escribe en 1.

### 9. Limpiar datos viejos

```bash
devherd observe cleanup --days 14
```

Elimina eventos, logs de contenedor, eventos de contenedor, entregas de alerta e issues
anteriores al corte indicado.

Lo que **no** limpia:

- El inventario de `containers`, que nunca caduca. Contenedores borrados hace meses siguen
  apareciendo en `observe containers` y en el panel.
- Las reglas de alerta (correcto: son configuracion).

Dos detalles del corte: los eventos se filtran por su `timestamp`, que **lo controla el
cliente**, asi que un SDK con el reloj desfasado puede no limpiarse nunca; y no se ejecuta
`VACUUM`, de modo que el archivo `.db` no se encoge. **No hay limpieza automatica**: hay
que ejecutar el comando a mano.

### 10. Desactivar Observe en el proyecto

```bash
devherd observe detach aang-server
```

Elimina `.devherd.observe.override.yml`. No toca codigo fuente ni configuracion de produccion.

## 11. Comandos y como funcionan

- `devherd observe start`: arranca el collector HTTP y el panel local en foreground. No hay
  daemon, ni pidfile, ni comando `stop`: se detiene interrumpiendo el proceso.
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
