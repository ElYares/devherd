# DevHerd Coverage

`devherd coverage` lee el reporte de cobertura que tu proyecto ya genera y calcula
sobre el lo que ninguna herramienta nativa te da: **donde esta la masa sin cubrir**.

> **DevHerd no instrumenta codigo.** No inyecta Xdebug, PCOV ni el agente de
> JaCoCo, y no cuenta lineas ejecutadas. Eso lo hace la herramienta de tu stack;
> aqui se lee lo que dejo. Configurar el proyecto para que lo genere sera
> `devherd coverage init`, que todavia no existe.

## Estado

**Fase 0 implementada.** Lee los cinco formatos, calcula y resume en terminal. La
ruta del reporte se pasa a mano con `--report`.

## Uso

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

## Limites de esta fase

- **La ruta se pasa a mano.** Encontrar el reporte solo, por stack, es la fase 3.
- **No corre las pruebas.** Es la fase 4.
- **No configura el proyecto.** Es la fase 1, `coverage init`.
- **No hay analisis estructural todavia.** El techo alcanzable —"el 69% de este
  paquete vive dentro de los `RunE`, tu maximo real es 31%"— es la fase 2. Necesita
  un analizador por lenguaje y empezara por Go.
- **Solo lineas y sentencias.** No se lee cobertura de ramas, aunque LCOV y JaCoCo
  la traigan.

## Un reporte vacio no es 0%

Si el reporte no tiene unidades medibles, la salida lo dice:

```text
  no coverage data: the report contains no measurable units
```

Mostrar `0.0%` seria mentir: se leeria como "nada esta probado", que es lo
contrario de "no hay nada medido".
