# Cross-Platform Runtime 2026-05-05

Esta nota cubre la siguiente fase de `devherd-v1-app`: abrir el runtime base para Linux, macOS y Windows.

## Alcance de esta pasada

- `doctor` ya no depende solo de `/proc/net/tcp`
- `open` ya no depende solo de `xdg-open`
- `ResolvePaths()` ya usa defaults mas razonables para macOS y Windows

## Cambios aplicados

### `doctor`

`internal/doctor/doctor.go` ahora usa estrategias por sistema operativo:

- Linux: `/proc/net/tcp` y `/proc/net/tcp6`, con fallback a `lsof`
- macOS: `lsof`
- Windows: `netstat -ano -p tcp`

Esto cubre los chequeos de puertos para:

- `devherd doctor`
- modo host (`80` y `443`)
- modo `caddy-docker-external` (`80`)

### `open`

`internal/cli/open.go` ahora selecciona el launcher por sistema:

- Linux: `xdg-open`
- macOS: `open`
- Windows: `cmd /c start`

Si el launcher no existe, el comando sigue imprimiendo la URL sin fallar duro.

### `ResolvePaths()`

`internal/config/paths.go` ahora resuelve defaults por sistema:

- Linux: sigue usando estilo XDG para data y state
- macOS: data bajo config dir del usuario y state bajo cache dir del usuario
- Windows: data bajo config dir del usuario y state bajo cache dir del usuario

### Docker Desktop y red compartida

`doctor` ahora endurece tambien estos puntos:

- valida que el engine Docker este en modo `linux`
- reporta el modo del engine como check separado
- advierte cuando el suffix no es `.localhost` en macOS o Windows
- inspecciona la red compartida del proxy (`driver`, `scope`, `internal`)

El runtime del proxy externo ahora crea la red compartida con:

- `--driver bridge`
- label `devherd.managed=true`
- label `devherd.role=shared-proxy`

Esto reduce sorpresas en Docker Desktop y deja un contrato mas claro para la red `infra_web`.

## Evidencia

- `go test ./...`: OK en Linux
- tests nuevos para parsing de `/proc/net/tcp`
- tests nuevos para parsing de `netstat` de Windows
- tests nuevos para selección de launcher en `open`
- tests nuevos para roots por OS en `ResolvePaths()`
- tests nuevos para engine Docker, suffix y metadata de red

## Lo que falta

Esta pasada deja el runtime base listo en código, pero todavía no equivale a validación completa en máquinas reales.

Pendiente de validación manual:

- macOS: `init`, `doctor`, `proxy bootstrap`, `park`, `up`, `proxy apply`, `open`
- Windows: `init`, `doctor`, `proxy bootstrap`, `park`, `up`, `proxy apply`, `open`

## Riesgos abiertos

- En Windows todavía falta validar interacción real con Docker Desktop y red compartida
- En macOS todavía falta validar el comportamiento real de `lsof` y `open`
- La documentación compartida sigue siendo Linux-first hasta que esta rama se consolide
