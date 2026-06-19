# Guia de contribucion a DevHerd

Como preparar el entorno, compilar, ejecutar tests, las convenciones de codigo que se
observan en el repositorio y como agregar un nuevo comando o feature.

## 1. Entorno de desarrollo

### Requisitos

- **Go** (el modulo declara `go 1.25.0` en `go.mod`; el repo se compila con toolchains
  recientes, p. ej. Go 1.26.x). Instala Go desde tu gestor (`mise`/`asdf`) o desde
  https://go.dev/dl/.
- **Docker** + `docker compose` para probar comandos que orquestan contenedores
  (`up`, `down`, `service`, `proxy apply`, `observe scan`).
- En modo de proxy en host: el binario **`caddy`** y acceso `sudo` (para `/etc/hosts` y
  recargar Caddy).

No se requiere CGO: el driver SQLite es `modernc.org/sqlite` (Go puro).

### Preparar el repo

```bash
git clone <repo> devherd
cd devherd
go mod tidy
go run ./cmd/devherd --help
```

## 2. Compilar

El repo trae un `Makefile` que es la via recomendada; inyecta los metadatos de version
(`version.Version/Commit/Date`) via `-ldflags`. El binario **no se versiona** (`/devherd`
esta en `.gitignore`); se compila a `bin/devherd`.

```bash
# Build con metadatos de version en bin/devherd
make build

# Instalar en $GOBIN con los mismos metadatos
make install

# Compilar y ejecutar (uso: make run ARGS="doctor")
make run ARGS="doctor"

# Otros targets utiles
make vet      # go vet ./...
make tidy     # go mod tidy
make clean    # borra bin/ y coverage.out
make help     # lista todos los targets

# Build manual (sin metadatos), o verificacion rapida de todos los paquetes
go build -o devherd ./cmd/devherd
go build ./...

# Instalar en ~/.local/bin (script de Ubuntu)
./scripts/install-ubuntu.sh
```

## 3. Ejecutar tests

El repo usa tests estandar de Go (`*_test.go`, ~20 archivos en paquetes como `cli`,
`compose`, `config`, `database`, `detector`, `dns`, `doctor`, `preflight`, `proxy`,
`services`, `observe`).

```bash
# Toda la suite con detector de carreras (target de Makefile)
make test

# Cobertura (genera coverage.out y resume el total)
make cover

# Equivalentes directos de go test:
go test ./...                                    # toda la suite
go test ./internal/proxy/...                     # un paquete
go test ./internal/cli/ -run TestNormalizeDomain -v   # un test concreto, verbose
go test -race ./...                              # con detector de carreras
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

> `coverage.out` y `*.db` estan en `.gitignore`; no los commitees.

### CI y linter

El repo corre **GitHub Actions** en cada push y pull request
(`.github/workflows/ci.yml`): `go vet`, `make build`, `go test -race` con cobertura y
**golangci-lint**. Por ser CGO-free (`modernc.org/sqlite`) la matriz es trivial.

El linter se configura en `.golangci.yml` con: `errcheck`, `govet`, `staticcheck`,
`ineffassign`, `unused`, `gocritic` y `revive`. Para correrlo localmente (requiere tener
`golangci-lint` instalado):

```bash
make lint     # equivalente a: golangci-lint run
```

Antes de enviar cambios (mismo conjunto que corre el CI):

```bash
make build        # o: go build ./...
make vet          # go vet ./...
gofmt -l .        # debe salir vacio (codigo ya formateado)
make test         # go test -race ./...
make lint         # golangci-lint run (si lo tienes instalado)
```

## 4. Convenciones de codigo observadas

Estas convenciones se infieren del codigo existente; mantenlas para consistencia.

### Estructura

- **`cmd/devherd/main.go`** es minimo: solo llama a `cli.Execute()` y mapea errores a
  `os.Exit(1)`. No agregues logica de comandos aqui.
- **`internal/cli/`** contiene un archivo por comando o grupo de comandos (p. ej.
  `up.go`, `proxy.go`, `observe.go`). Cada comando se define con una funcion
  `newXxxCmd() *cobra.Command`.
- **Logica de dominio fuera de `cli`**: cada area vive en su paquete
  (`internal/proxy`, `internal/compose`, `internal/database`, etc.). Los comandos en
  `cli` orquestan; la logica real esta en los paquetes de dominio. Sigue este patron.
- Paquetes con `doc.go` (p. ej. `internal/api`, `internal/logs`, `internal/runtimes`)
  son placeholders para iteraciones futuras.

### Estilo Cobra

- Patron de un comando (ver `internal/cli/list.go`):

  ```go
  func newListCmd() *cobra.Command {
      var asJSON bool
      cmd := &cobra.Command{
          Use:   "list",
          Short: "List registered projects",
          RunE: func(cmd *cobra.Command, args []string) error {
              // ...
          },
      }
      cmd.Flags().BoolVar(&asJSON, "json", false, "Output registered projects as JSON")
      return cmd
  }
  ```

- Usa `RunE` (no `Run`) y devuelve errores; nunca llames `os.Exit` dentro de comandos.
- Define `Args` explicitamente (`cobra.ExactArgs(1)`, `cobra.MaximumNArgs(1)`, etc.).
- Marca flags obligatorios con `cmd.MarkFlagRequired(...)`.
- Escribe salida a `cmd.OutOrStdout()` (no `fmt.Println` directo), para que los tests
  puedan capturarla.
- Los textos de `Short`/`Long` y los mensajes de flags estan en **ingles**; los mensajes
  de error tambien suelen estar en ingles. La documentacion de usuario (en `docs/`) esta
  en **espanol**.

### Acceso a config y base de datos

- Si tu comando necesita config + DB, usa `loadAppContext(cmd.Context())`
  (`internal/cli/app_context.go:20`) y haz `defer app.DB.Close()`.
- Si el comando puede funcionar sin inicializacion (como `inspect`/`doctor`), maneja el
  error de `loadAppContext` y cae a `config.Default()`.
- No abras SQLite manualmente: pasa por `database.NewManager(...)`.

### Errores y salida

- Envuelve errores con contexto: `fmt.Errorf("descripcion: %w", err)`.
- Para features no implementadas, usa `notImplemented("nombre")`
  (`internal/cli/root.go:47`).
- Comandos de ejecucion externa usan `exec.CommandContext` con timeouts cuando aplica
  (ver `internal/proxy/external.go:472` y `internal/doctor/doctor.go:455`).

### Tests

- Tests de tabla son el patron dominante (ver `internal/cli/naming_test.go`).
- Los tests viven junto al codigo (`xxx_test.go` en el mismo paquete).
- Para comandos Cobra, captura la salida con `cmd.SetOut(...)` y verifica el string
  resultante.
- Evita depender de Docker real en tests unitarios; usa funciones puras/aisladas cuando
  sea posible (p. ej. el parsing de `mergeManagedBlock`, `stripManagedDomains`,
  `procNetContainsListeningPort`).

### Formato

- Todo el codigo esta formateado con `gofmt` (tabs para indentacion). Ejecuta
  `gofmt -w .` antes de commitear.
- Mantente fiel a la API estandar de Go; las dependencias externas se limitan a Cobra,
  yaml.v3 y el driver SQLite.

## 5. Como agregar un nuevo comando

Ejemplo: agregar `devherd ping`.

1. **Crea el archivo** `internal/cli/ping.go`:

   ```go
   package cli

   import (
       "fmt"
       "github.com/spf13/cobra"
   )

   func newPingCmd() *cobra.Command {
       return &cobra.Command{
           Use:   "ping",
           Short: "Comprobacion rapida de salud",
           RunE: func(cmd *cobra.Command, args []string) error {
               fmt.Fprintln(cmd.OutOrStdout(), "pong")
               return nil
           },
       }
   }
   ```

2. **Registralo** en `internal/cli/root.go`, dentro de `cmd.AddCommand(...)`
   (`root.go:25`):

   ```go
   cmd.AddCommand(
       newInitCmd(),
       // ...
       newPingCmd(),
   )
   ```

3. **Si necesita config/DB**, usa `loadAppContext` (seccion 4) y mueve la logica real a un
   paquete de dominio (`internal/...`), no la pongas en `cli`.

4. **Agrega tests** en `internal/cli/ping_test.go`.

5. **Documenta** el comando en `docs/USAGE.md` (referencia de comandos) y, si cambia la
   arquitectura, en `docs/ARCHITECTURE.md`. Actualiza tambien `docs/cli-commands.md` y el
   `README.md` si listan el comando.

### Subcomandos anidados

Para grupos como `proxy`, `service`, `observe`, `sentry`, define un comando padre sin
`RunE` y agrega hijos con `AddCommand` (ver `internal/cli/proxy.go:15` o
`internal/cli/service.go:11`).

## 6. Como agregar una feature de dominio

- **Nuevo driver de proxy**: extiende `internal/proxy` y registra el valor en
  `applyInitOverrides` (`internal/cli/init.go:107`) y en los checks de
  `internal/doctor/doctor.go:53`.
- **Nuevo framework detectado**: extiende `featureSet` y `describeFramework` en
  `internal/detector/detector.go`, y si requiere rutas de proxy, agrega el caso en
  `Renderer.projectSite` (`internal/proxy/caddy.go:119`) y/o
  `BuildExternalProject` (`internal/proxy/external.go:90`).
- **Nuevo servicio compartido**: agregalo a `supportedServices` y a `composeContent` en
  `internal/services/manager.go`.
- **Cambios de esquema SQLite**: edita `internal/database/schema.sql` usando
  `CREATE TABLE IF NOT EXISTS` (el esquema se reaplica de forma idempotente en
  `Manager.Ensure`, `internal/database/db.go:21`).

## 7. Convenciones de Git y entorno

- `.gitignore` excluye `/devherd`, `bin/`, `dist/`, `coverage.out`, `*.db`, `*.log`. No
  commitees artefactos de build ni bases de datos locales. El binario se compila con
  `make build` a `bin/devherd`, no se versiona.
- El proyecto mantiene un grafo de conocimiento en `graphify-out/`. Tras modificar codigo,
  el flujo del proyecto sugiere `graphify update .` para mantenerlo actualizado
  (AST-only, sin costo de API). Ver `CLAUDE.md` en la raiz.
- Documentos de planificacion y estado viven en `docs/` (`technical-plan.md`,
  `current-status.md`, `cli-commands.md`, `project-workflow.md`, `observe.md`). Revisalos
  para contexto antes de cambios grandes.

## 8. Checklist antes de un PR

- [ ] `make build` (o `go build ./...`) compila sin errores.
- [ ] `gofmt -l .` no reporta archivos.
- [ ] `make vet` limpio.
- [ ] `make test` (`go test -race ./...`) en verde.
- [ ] `make lint` (golangci-lint) limpio.
- [ ] Comandos/flags nuevos documentados en `docs/USAGE.md`.
- [ ] Sin features inventadas: lo documentado coincide con el codigo.
</content>
