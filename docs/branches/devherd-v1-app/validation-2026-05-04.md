# Validación 2026-05-04

## Alcance

Esta validación cubre el bloque portable del proxy externo implementado en la rama `devherd-v1-app`.

## Resultado

- `go test ./...`: OK
- `devherd init --proxy caddy-docker-external` con XDG temporal: OK
- `devherd proxy bootstrap` con XDG temporal: OK
- `devherd proxy bootstrap --force` restaurando un `Caddyfile` dañado: OK

## Smoke test portable

Se ejecutó `init` con un root temporal XDG y el proxy se creó fuera del home personal:

```text
config: /tmp/devherd-v1-app-portable-cEhg7G/config/devherd/config.json
database: /tmp/devherd-v1-app-portable-cEhg7G/data/devherd/devherd.db
external proxy dir: /tmp/devherd-v1-app-portable-cEhg7G/data/devherd/local_proxy
```

Archivos generados:

- `.env`
- `.env.example`
- `Caddyfile`
- `docker-compose.yml`

Esto confirma que la rama ya no depende de `/home/elyarestark/infra/local_proxy`.

## Bootstrap idempotente

Una segunda ejecución de `devherd proxy bootstrap` sobre el mismo root temporal reportó:

- compose: `reused`
- caddyfile: `reused`
- env: `reused`
- env example: `reused`

## Recuperación con `--force`

Después de reemplazar el `Caddyfile` por contenido inválido, `devherd proxy bootstrap --force` reportó:

- compose: `reused`
- caddyfile: `updated`
- env: `reused`
- env example: `reused`

Y el `Caddyfile` volvió al template administrado por DevHerd.

## Lectura operativa

Con esta rama:

- la primera inicialización del proxy externo ya es portable
- la reparación básica del proxy administrado ya existe
- el runtime deja de depender de una ruta personal hardcodeada
- `park` evita duplicar stacks como `poderygozo-landing-page` en entradas hijas `backend` y `frontend` cuando la raíz ya define la orquestación

Lo que aún no se volvió a correr en esta rama es la validación completa contra repos reales después del cambio portable.
