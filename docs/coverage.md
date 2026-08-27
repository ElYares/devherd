# DevHerd Coverage

`devherd coverage` lee el reporte de cobertura que tu proyecto ya genera y calcula
sobre el lo que ninguna herramienta nativa te da: **donde esta la masa sin cubrir**.

> **DevHerd no instrumenta codigo.** No cuenta lineas ejecutadas ni reimplementa
> PHPUnit, JaCoCo, vitest o coverage.py. Lo que si hace `--run` es **habilitar** el
> driver que esas herramientas necesitan, correrlas y leer lo que dejaron.

## Estado

**Fases 0 y 1 implementadas.** Lee los cinco formatos y calcula (`--report`), y
prepara el contenedor, corre las pruebas y lee el resultado (`--run`).

## Uso rapido

```bash
devherd coverage --run                  # en la raiz del proyecto
devherd coverage --run --explain        # imprime los comandos, sin ejecutar nada
devherd coverage --run ~/proyectos/x    # otro proyecto
```

`--run` soporta **laravel** y **go**. Para los demas stacks, genera el reporte con
la herramienta del proyecto y pasalo con `--report`.

## Uso con un reporte ya existente

```bash
devherd coverage --report coverage.out          # Go
devherd coverage --report coverage/lcov.info    # Vue, React, TypeScript
devherd coverage --report clover.xml            # PHP / Laravel
devherd coverage --report target/site/jacoco/jacoco.xml   # Java
devherd coverage --report coverage.xml          # Python

devherd coverage --report coverage.out --all    # todos los archivos
devherd coverage --report coverage.out --top 5  # cuantos listar
devherd coverage --report coverage.out --json   # para encadenar
```

## Como generar el reporte

DevHerd solo lee. El reporte lo produce tu proyecto:

| Stack | Comando | Sale en |
|---|---|---|
| Go | `go test ./... -coverprofile=coverage.out` | `coverage.out` |
| PHP / Laravel | `php artisan test --coverage-clover=clover.xml` | `clover.xml` |
| Vue / React / TS | `vitest run --coverage` | `coverage/lcov.info` |
| Java (Maven) | `mvn test jacoco:report` | `target/site/jacoco/jacoco.xml` |
| Python | `pytest --cov --cov-report=xml` | `coverage.xml` |

PHP necesita **PCOV o Xdebug** instalado en el contenedor, y Java el plugin de
JaCoCo en el build. Sin eso no hay reporte que leer.

## Que se lee

Siete stacks, **cinco formatos**: Vue, React y TypeScript son el mismo ecosistema y
los tres emiten LCOV.

| Formato | De donde sale | Unidad |
|---|---|---|
| `go` | `go test -coverprofile` | sentencias |
| `lcov` | vitest, jest, c8, istanbul | lineas |
| `clover` | PHPUnit | sentencias |
| `jacoco` | Maven, Gradle | lineas |
| `cobertura` | coverage.py | lineas |

**El formato se detecta por el contenido, no por la extension**: un `jacoco.xml`
renombrado se sigue leyendo bien, y un `coverage.out` que no es de Go se rechaza en
vez de parsearse a medias.

## Salida

```text
coverage.out  ·  go  ·  statements

  directory                              covered        units
  ...ub.com/devherd/devherd/internal/cli   24.5%     365/1491
  ...om/devherd/devherd/internal/observe   64.9%     870/1341
  ...com/devherd/devherd/internal/runner  100.0%        17/17

  total                                    51.6%   (4982 statements)

  Largest uncovered mass:
    ...com/devherd/devherd/internal/cli/observe.go    429 uncovered   25.9%
    ...erd/devherd/internal/preflight/preflight.go    150 uncovered   38.3%
    57 more file(s) with uncovered statements (--all to list them)
```

## Tres cosas que hace distinto

### 1. La unidad va siempre en la cabecera

**Go cuenta sentencias; LCOV, JaCoCo y Cobertura cuentan lineas.** Un 58% de un
proyecto Go y un 58% de uno con vitest **no son comparables**, y sin la unidad a la
vista compararlos parece razonable.

Por eso `Report.Merge` **falla** si se le pasan dos reportes de unidades distintas,
en vez de dar un numero que no significa nada.

### 2. El total se pondera por unidades, nunca se promedian porcentajes

Un archivo de 3 lineas al 100% y otro de 800 al 40%:

- Promediando los porcentajes: **70%**
- Ponderando por unidades: **40,2%**

El segundo es el real. El primero es el error mas comun al hacer esto a mano.

### 3. Se ordena por masa sin cubrir, no por porcentaje

Un archivo de 800 unidades al 40% deja **480** sin cubrir; uno de 3 al 0% deja
**3**. Por porcentaje saldria primero el segundo, y no es donde esta el trabajo.

**Aplica igual a la tabla de directorios y a la de archivos**, y las dos se acotan
a `--top` (10 por defecto). Medido en un proyecto real de 38 directorios: en orden
alfabetico habia que leerlos todos para encontrar el bulto, que es justo el trabajo
que este comando viene a ahorrar.

Y nada **se trunca en silencio**: si se omiten filas, se dice cuantas. Con `--all`
salen todas.

## Que hace `--run`, paso a paso

```text
$ devherd coverage --run

tl-mas-server  (laravel)  ·  service app

  · coverage driver  pcov already present
  · memory limit     1G is enough
  ✓ tests            Tests:    865 passed (2203 assertions)

.devherd.coverage.xml  ·  clover  ·  statements
  total                                    78.1%   (2185 statements)
```

1. **Sondea el contenedor** una sola vez, de solo lectura: directorio de trabajo,
   usuario, si hay driver y cual es el limite de memoria.
2. **Instala PCOV si falta.** Es idempotente, cuesta ~3,5 s y **se pierde al
   recrear el contenedor**, por eso se re-verifica en cada corrida. Se usa PCOV y no
   Xdebug: aquel ralentiza las pruebas entre 2x y 20x, este ~1,3x.
3. **Sube `memory_limit` a 1G si esta por debajo de 512M.** Con los 128M por
   defecto, PCOV mata la suite a mitad. **Se escribe en `conf.d`, nunca con `-d`**:
   `artisan test` lanza el runner en un proceso hijo que no hereda los flags.
4. **Corre las pruebas** y lee el reporte.

Los pasos que ya estaban resueltos **se anuncian igual** (con `·` en vez de `✓`):
saber que algo no hacia falta vale tanto como haberlo hecho.

### `--explain`: automatico no quiere decir opaco

```bash
devherd coverage --run --explain
```

Imprime los comandos exactos y **no ejecuta ninguno**. Sirven para leerlos, para
copiarlos y correrlos a mano, o para entender que va a pasar antes de dejar actuar
al comando.

### Detalles que conviene saber

- **`-u root` solo cuando hace falta.** `aang-server` corre como uid 1000 y ahi
  `pecl install` falla por permisos; `tl-mas-server` ya es root y forzarlo cambiaria
  el dueno de lo que escriba. Las **pruebas siempre corren con el usuario
  original**, nunca como root.
- **El comando de pruebas no se puede adivinar.** El de por defecto para Laravel es
  `php artisan test`, **no** `vendor/bin/phpunit`: los proyectos con Pest revientan
  con un error de bootstrap. Se declara asi:

  ```yaml
  # .devherd.yml
  test:
    command: php artisan test --coverage-clover=.devherd.coverage.xml
    service: app
  ```

- **Si las pruebas fallan**, el aviso sale por stderr **antes** del resumen, el
  numero **se muestra igual** —la cobertura de lo que si corrio es real, y la
  corrida ya costo su tiempo— y se sale con codigo distinto de cero.
- **Si la suite muere sin generar reporte**, se dice eso y no "sin datos de
  cobertura": son cosas distintas.
- **El reporte anterior se borra antes de empezar.** Si no, una corrida que muere
  dejaria leer el de la vez pasada como si fuera de ahora.
- **El reporte va dentro del proyecto**, como `.devherd.coverage.*`. No puede ir
  fuera: el contenedor solo ve el proyecto montado. El comando avisa si no esta en
  el `.gitignore`.

## Limites

- **`--run` solo sabe de laravel y go.** Node, Python y Java se agregan como
  adaptadores; mientras tanto, `--report`.
- **No encuentra un reporte ya existente.** Pasar la ruta a mano sigue siendo
  necesario sin `--run`. Es la fase 3.
- **No hay analisis estructural todavia.** El techo alcanzable —"el 69% de este
  paquete vive dentro de los `RunE`, tu maximo real es 31%"— es la fase 2. Necesita
  un analizador por lenguaje y empezara por Go.
- **No toca Dockerfiles ni imagenes.** Instala en el contenedor vivo.
- **Solo lineas y sentencias.** No se lee cobertura de ramas, aunque LCOV y JaCoCo
  la traigan.

## Un reporte vacio no es 0%

Si el reporte no tiene unidades medibles, la salida lo dice:

```text
  no coverage data: the report contains no measurable units
```

Mostrar `0.0%` seria mentir: se leeria como "nada esta probado", que es lo
contrario de "no hay nada medido".
