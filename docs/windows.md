# DevHerd en Windows

Esta guia cubre el soporte inicial de DevHerd en Windows. El camino recomendado para Windows nativo usa Docker Desktop con Linux containers y el proxy `caddy-docker-external`.

## Soporte objetivo

- Windows 10/11 con Docker Desktop.
- Docker Desktop en modo Linux containers.
- Docker Compose plugin disponible desde `docker compose`.
- Dominios `.localhost` mediante el proxy Docker externo de DevHerd.
- WSL2 sigue siendo soportado como entorno Linux: instala DevHerd dentro de la distro y usa Docker Desktop con integracion WSL.

El modo `caddy` instalado en el host no es el camino recomendado en Windows. Usa `caddy-docker-external`.

## Instalacion nativa

Desde PowerShell, en la raiz del repositorio:

```powershell
.\scripts\install-windows.ps1 -AddToPath
```

El binario se instala en:

```text
%LOCALAPPDATA%\Programs\DevHerd\devherd.exe
```

Abre una terminal nueva y valida:

```powershell
devherd --version
devherd --help
```

Para desinstalar:

```powershell
.\scripts\uninstall-windows.ps1 -RemoveFromPath
```

## Rutas locales

En Windows nativo, DevHerd usa rutas del usuario:

```text
config: %APPDATA%\devherd\config.json
data:   %LOCALAPPDATA%\devherd
db:     %LOCALAPPDATA%\devherd\devherd.db
proxy:  %LOCALAPPDATA%\devherd\local_proxy
```

## Primer arranque nativo

En Windows, `devherd init` usa `caddy-docker-external` y `.localhost` por defecto.

```powershell
devherd init
devherd doctor
```

La salida esperada de `doctor` debe incluir checks OK para:

- Docker CLI
- Docker daemon
- Docker engine mode: Linux containers
- Docker Compose
- local_proxy
- proxy network
- managed suffix `.localhost`

Si Docker Desktop esta apagado o en Windows containers, `doctor` debe fallar y decirlo antes de levantar proyectos.

## Flujo de proyecto

```powershell
devherd park C:\Users\tu-usuario\develop\projects
devherd list
devherd plan C:\Users\tu-usuario\develop\projects\mi-proyecto
devherd up C:\Users\tu-usuario\develop\projects\mi-proyecto
devherd proxy apply mi-proyecto
devherd open mi-proyecto
```

Para apagar:

```powershell
devherd down C:\Users\tu-usuario\develop\projects\mi-proyecto
```

## Observe

El collector local funciona igual que en Linux:

```powershell
devherd observe start
devherd observe status
devherd observe open
```

Para adjuntar un proyecto Docker Compose:

```powershell
devherd observe attach C:\Users\tu-usuario\develop\projects\mi-proyecto --stack node --service web
devherd up C:\Users\tu-usuario\develop\projects\mi-proyecto
devherd observe scan mi-proyecto
devherd observe containers mi-proyecto
```

## WSL2

En WSL2, instala DevHerd dentro de la distro Linux:

```bash
./scripts/install-ubuntu.sh
devherd init --proxy caddy-docker-external
devherd doctor
```

Recomendaciones:

- Clona proyectos dentro del filesystem Linux de WSL, no bajo `/mnt/c`, para evitar I/O lento.
- Activa la integracion de Docker Desktop con tu distro WSL.
- Usa `.localhost` para evitar configuracion DNS manual.

## Limitaciones actuales

- Windows nativo todavia requiere validacion manual completa con proyectos reales.
- El modo host `caddy` no es el flujo oficial en Windows.
- Si el puerto `80` ya esta ocupado, `doctor` lo reporta; libera el puerto o ajusta la configuracion antes de usar `proxy apply`.
- Si cambias el TLD a algo distinto de `.localhost`, puede requerir DNS o hosts manual.
