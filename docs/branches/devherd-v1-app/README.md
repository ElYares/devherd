# devherd-v1-app

Este árbol documenta únicamente el trabajo de la rama `devherd-v1-app`.

No reemplaza la documentación compartida actual del repo. Mientras esta rama no se integre, los archivos de `docs/` en la raíz pueden seguir describiendo el flujo viejo basado en `/home/elyarestark/infra/local_proxy`.

## Objetivo

Volver portable el modo `caddy-docker-external` para que cualquier máquina pueda usarlo sin depender de una carpeta personal preexistente.

## Contrato portable actual

`devherd init --proxy caddy-docker-external` ahora persiste estos campos en config:

```json
{
  "proxy": {
    "driver": "caddy-docker-external",
    "external_dir": "~/.local/share/devherd/local_proxy",
    "external_network": "infra_web",
    "external_container_name": "infra_caddy"
  }
}
```

Notas:

- `external_dir` realmente se resuelve vía XDG data dir. En una máquina Linux estándar termina en `~/.local/share/devherd/local_proxy`.
- `external_network` y `external_container_name` tienen defaults portables, pero ya no están hardcodeados en el runtime.

## Fases cerradas en esta rama

- Fase 1: el contrato portable ya existe en `internal/config/config.go`.
- Fase 2: el `local_proxy` ahora vive como templates embebidos en `templates/proxy-external/`.
- Fase 3: `devherd init --proxy caddy-docker-external` ya crea `docker-compose.yml`, `Caddyfile`, `.env` y `.env.example` dentro de `external_dir`.
- Fase 4: `proxy apply`, `down`, `open` y `doctor` ya leen config en vez de depender de `/home/elyarestark/infra/local_proxy`.
- Fase 5: `doctor` ya valida `external_dir`, archivos base, Docker, red compartida y puerto 80 para el proxy externo.
- Fase 6: existe `devherd proxy bootstrap` para reparar o regenerar los assets administrados del proxy.
- Fase 7: `park` ya filtra subproyectos redundantes cuando la raíz tiene orquestación propia y limpia registros viejos bajo esa ruta.

## Comandos nuevos o ajustados

```bash
devherd init --proxy caddy-docker-external
devherd proxy bootstrap
devherd proxy bootstrap --force
devherd doctor
```

Comportamiento:

- `init` hace bootstrap inicial y no sobreescribe assets existentes.
- `proxy bootstrap` vuelve a asegurar que existan los assets administrados.
- `proxy bootstrap --force` reescribe `docker-compose.yml`, `Caddyfile` y `.env.example` para alinearlos con la config actual.
- `proxy bootstrap --force` preserva `.env` para no romper overrides locales del usuario.
- `proxy apply` y `down` ya pueden autocrear los archivos base si faltan.

## Evidencia

La validación de esta rama quedó registrada en [validation-2026-05-04.md](./validation-2026-05-04.md).

## Pendiente

- Revalidar el ciclo completo `up -> park -> proxy apply -> open -> down` con proyectos reales usando un binario construido desde esta rama.
- Decidir si conviene agregar un `devherd proxy doctor` específico o si `devherd doctor` ya es suficiente.
- Cuando esta rama se consolide, migrar la documentación compartida para retirar las referencias al `local_proxy` personal viejo.
