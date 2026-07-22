# DevHerd CLI: referencia de comandos

> **Este documento se consolido en [USAGE.md](USAGE.md).**
>
> La referencia de comandos, flags, valores por defecto y efectos de cada comando vive
> ahora en un unico sitio, para que no haya dos versiones que se contradigan.

## Donde esta ahora cada cosa

| Lo que buscabas | Donde esta |
|---|---|
| Todos los comandos, flags y ejemplos | [USAGE.md, seccion 4](USAGE.md#4-referencia-de-comandos) |
| Flags globales (`--verbose`, `--log-json`) | [USAGE.md, seccion 3](USAGE.md#3-flags-globales) |
| El manifiesto `.devherd.yml` | [USAGE.md, seccion 5](USAGE.md#5-el-manifiesto-devherdyml) |
| Flujos de trabajo completos | [USAGE.md, seccion 6](USAGE.md#6-flujos-de-trabajo) |
| Donde vive el estado en disco | [USAGE.md, seccion 7](USAGE.md#7-donde-vive-el-estado) |
| Aislamiento con `COMPOSE_NAME_PREFIX` y volumenes | [USAGE.md, seccion 8](USAGE.md#8-patrones-recomendados-para-proyectos-compose) |
| Flujos narrativos paso a paso | [project-workflow.md](project-workflow.md) |
| Arquitectura interna y paquetes | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Estado real del sistema y deuda tecnica | [SYSTEM-OVERVIEW.md](SYSTEM-OVERVIEW.md) |
| Observabilidad local en detalle | [observe.md](observe.md) |
| Generacion de compose | [guides/scaffold.md](guides/scaffold.md) |

## Por que se consolido

Este archivo describia los mismos comandos que `USAGE.md` y `project-workflow.md`, y con
el tiempo las tres versiones se desincronizaron. Entre otras cosas, aqui se afirmaba que
`devherd logs` "aun devuelve not implemented" cuando llevaba tiempo implementado, y no se
documentaban `devherd scaffold` ni `devherd serve`.

El contenido historico sigue disponible en el historial de git.
