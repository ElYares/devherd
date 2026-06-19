# Scaffolding de Docker para repos sin contenedores

`devherd scaffold` detecta el stack de un proyecto que **no trae** `docker-compose.yml`
ni `Dockerfile`, genera un compose dev-friendly (+ manifiesto `.devherd.yml`) y te
deja listo para `devherd up`.

```
repo sin Docker  →  devherd scaffold  →  detecta stack
                 →  genera docker-compose.devherd.yml + .devherd.yml
                 →  devherd up
```

## Stacks soportados

| Layout | Imagen base | Puerto por defecto |
|---|---|---|
| `vue+flask` (hijo Flask + hijo Vue) | `python:3.12-slim` + `node:20-alpine` | 8000 / 5173 (fijos, contrato del proxy) |
| `laravel` (artisan + composer.json) | `php:8.3-cli` | 8000 |
| `vue` | `node:20-alpine` | 5173 |
| `flask` | `python:3.12-slim` | 8000 |
| `node` | `node:20-alpine` | 3000 |
| `go` | `golang:1.25` | 8080 |

El compose generado usa **imágenes base + bind-mount del código** (sin necesidad de
Dockerfiles): rápido para desarrollo y editable.

## Detección fina

Para stacks de un solo servicio, DevHerd respeta lo que el repo ya declara:
- **Puerto**: lo lee del `.env` (`PORT`, `APP_PORT`, `FLASK_RUN_PORT`); si no, usa el default del stack.
- **Comando**: elige el script real de `package.json` (`dev` > `serve` > `start`).
- El `.devherd.yml` generado declara `proxy.service`/`port` para que el proxy enrute correctamente.

## Bases de datos y Redis

Al generar, DevHerd te pregunta por la base de datos (o usa el flag `--db`):

```
¿Qué base de datos quieres?
  1) MySQL        2) MariaDB      3) PostgreSQL
  4) MongoDB      5) Ninguna
```

- **Redis** se incluye por defecto (la mayoría de proyectos lo usan); quítalo con `--redis=false`.
- La base de datos y Redis quedan **internos a la red del proyecto**: la app se conecta
  por nombre de servicio (`db:3306`, `redis:6379`) y DevHerd cablea las variables de
  entorno de conexión (`DB_HOST`, `DB_PORT`, `REDIS_HOST`, `DATABASE_URL`, etc.) y el
  `depends_on`. La DB usa un **named volume** para persistir.

## Sin colisiones con tus proyectos levantados

Los servicios de aplicación se publican en **puertos de host libres autodetectados**:
si el `5173` ya lo ocupa otro proyecto, DevHerd usa `5174`. Las bases de datos y Redis
**no publican puertos**, así que nunca chocan con contenedores existentes.

## Uso

```bash
# Genera el compose (te pregunta la base de datos)
devherd scaffold ~/dev/mi-app

# Sin interacción
devherd scaffold ~/dev/mi-app --db postgres
devherd scaffold ~/dev/mi-app --db mysql --redis=false

# Previsualiza sin escribir nada
devherd scaffold ~/dev/mi-app --dry-run

# Genera y levanta
devherd scaffold ~/dev/mi-app --db mysql --up
```

### Flags

| Flag | Descripción |
|---|---|
| `--db <kind>` | `mysql`, `mariadb`, `postgres`, `mongodb`, `none` (si se omite, pregunta) |
| `--redis` | Incluir Redis (por defecto `true`; `--redis=false` para omitirlo) |
| `--dry-run` | Muestra el compose y el manifiesto sin escribir archivos |
| `--up` | Genera y luego ejecuta `devherd up` |
| `--force` | Sobrescribe archivos generados existentes |

## Oferta automática desde `up`

Si ejecutas `devherd up` en un proyecto sin compose pero con un stack detectable,
DevHerd te ofrece generarlo en el acto:

```
$ devherd up ~/dev/mi-app
No encontré un docker-compose en este proyecto.
¿Genero uno con scaffold? [Y/n]
```

## Archivos generados

| Archivo | Qué es |
|---|---|
| `docker-compose.devherd.yml` | El compose generado (nombre propio para no pisar un `docker-compose.yml` tuyo) |
| `.devherd.yml` | Manifiesto que apunta al compose y declara el proxy para stacks de un servicio |

La escritura es **no destructiva**: si los archivos ya existen, no se sobrescriben
(usa `--force` para regenerarlos).
