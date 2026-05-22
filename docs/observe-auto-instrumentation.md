# DevHerd Observe: Auto-Instrumentacion

Este documento lista las fases pendientes para que DevHerd Observe capture errores reales de un proyecto sin depender de eventos manuales con `curl`.

El objetivo es que un flujo como este sea suficiente:

```bash
devherd observe start
devherd observe attach . --service api --stack python
devherd observe attach . --service web --stack node
devherd up .
devherd observe open
```

## Fase 1: Instrumentar API

Para backends Python/FastAPI:

- agregar SDK Sentry Python en el proyecto
- inicializarlo solo si existe `SENTRY_DSN`
- asegurar que el boot no falle si Observe no esta corriendo
- capturar excepciones no controladas automaticamente
- enviar metadata minima:
  - `service=api`
  - `environment=local`
  - `release`
  - `transaction`
- validar que un error real en un endpoint aparezca en:

```bash
devherd observe events <project>
devherd observe issues <project>
```

## Fase 2: Instrumentar Web

Para frontends Next.js:

- agregar SDK Sentry para Next.js
- configurar captura client/server/edge segun aplique
- usar variables inyectadas por DevHerd:
  - `SENTRY_DSN`
  - `NEXT_PUBLIC_SENTRY_DSN`
  - `SENTRY_ENVIRONMENT`
  - `DEVHERD_PROJECT`
- capturar errores de:
  - render server-side
  - rutas/API routes si existen
  - errores del navegador
  - errores no controlados de React/Next
- enviar metadata minima:
  - `service=web`
  - `environment=local`
  - `release`
  - `transaction`

## Fase 3: Mejorar `observe attach`

`observe attach` debe manejar stacks mixtos sin pisar servicios previos.

Flujo deseado:

```bash
devherd observe attach . --service api --stack python
devherd observe attach . --service web --stack node
```

Comportamiento esperado:

- conservar servicios ya adjuntados en `.devherd.observe.override.yml`
- permitir stacks distintos por servicio
- para `python`, inyectar variables backend:
  - `SENTRY_DSN`
  - `SENTRY_ENVIRONMENT`
  - `DEVHERD_OBSERVE`
  - `DEVHERD_PROJECT`
  - `DEVHERD_OBSERVE_STACK`
- para `node`/Next.js, inyectar tambien:
  - `NEXT_PUBLIC_SENTRY_DSN`
  - `NEXT_PUBLIC_DEVHERD_PROJECT`
- agregar labels Docker:
  - `devherd.observe=true`
  - `devherd.project=<project>`
  - `devherd.service=<service>`
  - `devherd.stack=<stack>`

## Fase 4: Endpoints/Rutas de prueba dev-only

Agregar una forma de provocar errores reales sin usar `curl` contra Observe.

API:

- endpoint dev-only que lance una excepcion
- activo solo con `DEVHERD_OBSERVE=1` o `APP_ENV=development`

Web:

- ruta o boton dev-only que lance error client-side
- ruta o accion dev-only que lance error server-side
- activo solo con `DEVHERD_OBSERVE=1` o `NODE_ENV=development`

Esto permite validar que el SDK real esta enviando eventos al collector local.

## Fase 5: Correlacion con Docker

Los eventos deben poder asociarse al contenedor correcto.

Requisitos:

- eventos con `service`
- labels Docker por servicio
- snapshots recientes de contenedores observados
- logs cercanos al timestamp del evento
- timeline que una:
  - evento
  - issue
  - contenedor
  - eventos de contenedor
  - logs cercanos

Comandos esperados:

```bash
devherd observe scan <project>
devherd observe containers <project>
devherd observe timeline <event-id>
```

## Fase 6: Panel

El panel local debe mostrar claramente el estado de Observe.

Pendientes:

- filtro por proyecto
- filtro por servicio
- lista de contenedores observados
- estado de contenedor observado sin eventos
- issues agrupados
- eventos recientes
- timeline con logs cercanos
- indicador cuando el SDK no ha enviado eventos todavia

## Fase 7: Comando de prueba

Agregar un comando para validar Observe sin escribir `curl` manual.

Ejemplo deseado:

```bash
devherd observe test <project> --service api
devherd observe test <project> --service web
```

El comando debe:

- enviar un evento sintetico al collector o disparar una ruta dev-only si esta disponible
- confirmar que el evento fue almacenado
- mostrar `event_id`, `issue_id`, `service` y `timeline`

## Fase 8: Documentacion

Documentar el flujo final para proyectos reales:

```bash
devherd observe start
devherd observe attach . --service api --stack python
devherd observe attach . --service web --stack node
devherd down .
devherd up .
devherd observe scan <project>
devherd observe open
```

Tambien documentar:

- que errores se capturan automaticamente
- que errores requieren instrumentacion manual
- como validar API y Web por separado
- como desactivar Observe:

```bash
devherd observe detach .
```

## Orden recomendado

1. Instrumentar API.
2. Instrumentar Web.
3. Mejorar `observe attach` para stacks por servicio.
4. Agregar rutas dev-only de prueba.
5. Mejorar correlacion y timeline.
6. Mejorar panel.
7. Agregar `devherd observe test`.
8. Documentar el flujo final.
