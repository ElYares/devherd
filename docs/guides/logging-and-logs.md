# Logging de diagnóstico y comando `logs`

Esta guía documenta tres funcionalidades nuevas de **devherd** que mejoran la
observabilidad de la herramienta y de los proyectos que administra:

1. **Logging estructurado de diagnóstico con `slog`**, controlado por flags
   globales (`--verbose`, `--log-json`).
2. **El comando `devherd logs [path]`**, que transmite los logs de un proyecto.
3. **Manejo robusto de errores en el colector `observe`**, que antes se tragaba
   ciertos fallos silenciosamente.

Una idea atraviesa las tres: devherd distingue entre **salida de producto**
(resultados que el usuario pidió, en `stdout`) y **diagnósticos** (qué está
haciendo la herramienta por dentro, en `stderr`). Esa separación permite seguir
canalizando `stdout` (por ejemplo `devherd list --json | jq`) sin que el ruido
de diagnóstico contamine el flujo de datos.

---

## 1. Logging estructurado con `slog`

El logger global de diagnóstico se configura en
`internal/cli/logging.go` y se cablea en `internal/cli/root.go`.

### Cómo funciona

El struct `logOptions` (`internal/cli/logging.go:9`) guarda las dos opciones
expuestas como flags persistentes, y `setupLogging`
(`internal/cli/logging.go:18`) construye el handler de `slog` correspondiente:

- Nivel por defecto: `slog.LevelInfo`.
- Con `--verbose` el nivel baja a `slog.LevelDebug`
  (`internal/cli/logging.go:20-22`).
- Con `--log-json` se usa `slog.NewJSONHandler`; en caso contrario,
  `slog.NewTextHandler` (`internal/cli/logging.go:27-31`).
- **En ambos casos el handler escribe a `os.Stderr`**
  (`internal/cli/logging.go:28` y `:30`), nunca a `stdout`.

El cableado vive en el comando raíz. Los flags se declaran como persistentes
para que estén disponibles en cualquier subcomando
(`internal/cli/root.go:30-31`), y `setupLogging` se invoca desde
`PersistentPreRunE`, de modo que el logger queda configurado antes de ejecutar
cualquier subcomando (`internal/cli/root.go:24-27`).

### Modelo: diagnósticos a stderr, producto a stdout

Los mensajes de diagnóstico (qué está haciendo devherd, advertencias, errores
recuperables) se emiten con `slog` a **stderr**. La salida "de producto" —lo
que el usuario realmente pidió— sigue yendo a **stdout** a través de
`cmd.OutOrStdout()`. Gracias a esta separación puedes redirigir cada flujo por
separado.

### Por qué `--log-json` y no `--json`

El nombre del flag es `--log-json`, no `--json`, de forma deliberada: el
subcomando `list` ya define su propio flag `--json` para emitir la lista de
proyectos como JSON en stdout (`internal/cli/list.go:13`, `:31-39`). Si el flag
global de logging se llamara `--json`, colisionaría con ese flag local de
`list`. Usar `--log-json` evita la ambigüedad y deja claro que el flag afecta a
los **logs de diagnóstico**, no a la salida de producto.

### Flags

| Flag          | Tipo | Default | Efecto                                                        |
| ------------- | ---- | ------- | ------------------------------------------------------------ |
| `--verbose`   | bool | `false` | Baja el nivel de diagnóstico a `DEBUG` en stderr.            |
| `--log-json`  | bool | `false` | Emite los logs de diagnóstico en formato JSON en stderr.    |

Ambos son **persistentes**: aplican a cualquier subcomando de devherd.

### Ejemplos

```bash
# Diagnóstico detallado de cualquier comando (logs en stderr, en texto)
devherd --verbose up

# Logs de diagnóstico en JSON, útil para agregadores o scripting
devherd --log-json up

# Combinar ambos: DEBUG en JSON
devherd --verbose --log-json observe serve

# Separar producto (stdout) de diagnóstico (stderr):
# la lista JSON queda limpia para jq, el diagnóstico va al archivo
devherd --verbose list --json > projects.json 2> diag.log
```

---

## 2. Comando `devherd logs [path]`

El comando `logs` antes era un *stub* que devolvía "not implemented". Ahora está
implementado en `internal/cli/logs.go` y transmite (`tail`/`follow`) los logs de
un proyecto, apoyándose en `docker compose logs`.

### Cómo funciona

El comando se construye en `newLogsCmd` (`internal/cli/logs.go:12`) y acepta
como máximo un argumento de ruta (`cobra.MaximumNArgs(1)`,
`internal/cli/logs.go:21`). Si no se pasa ruta, se resuelve el proyecto del
directorio actual (`internal/cli/logs.go:23-31`).

Un detalle importante: el comando **alinea los compose files con los que usa
`up`** (`internal/cli/logs.go:33-46`). Cuando hay un app context disponible:

- Si el proyecto usa el proxy externo de Docker
  (`proxy.UsesDockerExternal`), añade el override del proxy gestionado si
  existe (`internal/cli/logs.go:39-44`).
- Añade el override de `observe` vía `appendObserveOverride`
  (`internal/cli/logs.go:45`).

Así los logs cubren **todos** los servicios en ejecución y no solo los del
compose base. El app context es opcional: sin él, se usa el proyecto base.

Finalmente delega en el helper de compose (`internal/cli/logs.go:48-53`),
pasando explícitamente `cmd.OutOrStdout()` y `cmd.ErrOrStderr()`.

### El helper en `internal/compose/logs.go`

El paquete `compose` expone una API limpia y testeable:

- `LogsOptions` (`internal/compose/logs.go:10`): struct con `Follow`, `Tail` y
  `Services`.
- `LogsArgs` (`internal/compose/logs.go:18`): **función pura** que construye los
  argumentos de `docker compose ... logs ...`. Al no tocar Docker, se puede
  testear sin contenedores. Traduce `Follow` a `--follow`
  (`:22-24`) y `Tail` a `--tail <valor>` (`:25-27`).
- `LogsProject` (`internal/compose/logs.go:36`): ejecuta el comando y, a
  diferencia de otras rutas que almacenan la salida en buffer, **conecta los
  writers directamente** (`cmd.Stdout = stdout`, `cmd.Stderr = stderr`,
  `:41-42`). Esto es lo que permite el streaming en vivo de `--follow` sin
  bufferizar.
- `Logs` (`internal/compose/logs.go:48`): conveniencia que resuelve el proyecto
  desde una ruta y luego llama a `LogsProject`.

### Flags

| Flag             | Alias | Tipo   | Default | Efecto                                                             |
| ---------------- | ----- | ------ | ------- | ----------------------------------------------------------------- |
| `--follow`       | `-f`  | bool   | `false` | Sigue la salida de logs en vivo (streaming).                      |
| `--tail`         | —     | string | `""`    | Muestra las últimas N líneas (p. ej. `100`) o `all` para todas.   |

### Ejemplos

```bash
# Logs del proyecto en el directorio actual
devherd logs

# Logs de un proyecto indicando su ruta
devherd logs ~/develop/personal/mi-proyecto

# Seguir los logs en vivo (Ctrl+C para salir)
devherd logs --follow
devherd logs -f

# Últimas 100 líneas y luego seguir en vivo
devherd logs --tail 100 -f

# Todas las líneas históricas de un proyecto por ruta
devherd logs --tail all ~/develop/personal/mi-proyecto
```

---

## 3. Manejo de errores en el colector `observe`

En `internal/observe/server.go` se corrigieron tres puntos donde antes ciertos
errores se **tragaban silenciosamente**. Ahora todos se registran con
`slog.Warn` (nivel `WARN`) incluyendo contexto estructurado, de modo que el
colector sigue operando pero deja rastro de lo que falló.

Los tres puntos son:

1. **Correlación de eventos con logs de contenedor**
   (`internal/observe/server.go:139-142`). Si `correlator.CorrelateEvent`
   falla, se loguea con los campos `project`, `container` y `error`. El evento
   se sigue almacenando aunque no se haya podido correlacionar.

2. **Almacenamiento de logs de contenedor**
   (`internal/observe/server.go:151-154`). Si `StoreContainerLogs` falla, se
   loguea con `event_id`, `log_count` y `error`.

3. **Listado y almacenamiento de contenedores observados**
   (`internal/observe/server.go:186-198`). En `snapshotObservedContainers`:
   - Si `ObservedContainers` falla al **listar**, se loguea con `project` y
     `error`, y la función retorna temprano (`:187-190`).
   - Si `StoreContainers` falla al **almacenar**, se loguea con `project`,
     `count` y `error` (`:195-198`).

En los tres casos el nivel elegido es `WARN`: son fallos recuperables que no
deben abortar el colector (que corre en bucle vía `pollObservedContainers`,
`internal/observe/server.go:167-179`), pero que ya no pasan inadvertidos.
Recuerda que estos logs van a **stderr**, igual que el resto de diagnósticos.

---

## Cómo ver los logs de diagnóstico

Como todos los diagnósticos van a `stderr`, la forma más cómoda de capturarlos
sin contaminar la salida de producto es redirigir solo `stderr`:

```bash
# Ejecuta un comando con diagnóstico DEBUG y guarda solo el diagnóstico
devherd --verbose <cmd> 2> diag.log

# Ejemplo concreto: arrancar un proyecto y revisar el diagnóstico aparte
devherd --verbose up 2> diag.log

# Diagnóstico en JSON para parsearlo con jq
devherd --verbose --log-json up 2> diag.json
cat diag.json | jq 'select(.level == "WARN")'

# Ver solo el diagnóstico en pantalla, descartando la salida de producto
devherd --verbose list 1> /dev/null
```

Para el colector `observe`, que corre como servidor de larga duración, conviene
ejecutarlo con `--verbose` y persistir su stderr para auditar las advertencias
de correlación y almacenamiento descritas arriba:

```bash
devherd --verbose observe serve 2> observe-diag.log
```
