# Vikunja con DevHerd

Esta guia deja un flujo minimo para levantar Vikunja con `devherd` usando Docker Compose y SQLite.

## 1. Crear el proyecto

Primero crea el directorio del proyecto:

```bash
mkdir -p ~/apps/vikunja
cd ~/apps/vikunja
```

Luego crea `docker-compose.yml` con este contenido:

```yaml
services:
  vikunja:
    image: vikunja/vikunja:latest
    container_name: vikunja
    restart: unless-stopped
    ports:
      - "3456:3456"
    environment:
      VIKUNJA_SERVICE_PUBLICURL: http://localhost:3456
      VIKUNJA_DATABASE_TYPE: sqlite
    volumes:
      - ./files:/app/vikunja/files
```

Ese volumen local guarda los datos de SQLite y los archivos de Vikunja dentro de `~/apps/vikunja/files`.

## 2. Levantarlo con DevHerd

Desde la raiz del proyecto:

```bash
devherd up ~/apps/vikunja
```

Si ya estas dentro de la carpeta del proyecto, tambien puedes ejecutar:

```bash
devherd up
```

DevHerd detecta el `docker-compose.yml`, valida el stack antes de levantarlo y ejecuta `docker compose up --build -d`.

## 3. Abrir Vikunja

Una vez levantado, abre:

```text
http://localhost:3456
```

## 4. Tumbarlo cuando no lo necesites

Si solo quieres detener el contenedor sin borrar el stack:

```bash
devherd stop ~/apps/vikunja
```

Si quieres bajar todo el stack Compose:

```bash
devherd down ~/apps/vikunja
```

Usa `stop` si piensas reanudarlo despues. Usa `down` si quieres limpiar el stack completo.

## 5. Reanudarlo

Si lo detuviste con `stop`, vuelve a levantarlo con:

```bash
devherd up ~/apps/vikunja
```

## 6. Comandos rapidos

```bash
devherd up ~/apps/vikunja
devherd stop ~/apps/vikunja
devherd down ~/apps/vikunja
```

## 7. Nota sobre datos

Con la definicion actual, Vikunja usa SQLite y persiste datos en `~/apps/vikunja/files`.
Si borras ese directorio, perderas los datos locales de la instancia.
