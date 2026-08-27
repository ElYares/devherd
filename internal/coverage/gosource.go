package coverage

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Module es un modulo de Go localizado en disco: su ruta de import declarada y el
// directorio que la contiene. Los dos datos juntos son lo que traduce las rutas
// del perfil, que vienen como import paths, a archivos que se pueden abrir.
type Module struct {
	// Path es lo que declara la directiva `module` del go.mod.
	Path string
	// Dir es el directorio donde vive ese go.mod.
	Dir string
}

// maxModuleSearchDepth acota la busqueda hacia arriba. Sin tope, un directorio
// sin go.mod hace subir hasta la raiz del sistema de archivos leyendo en cada
// nivel, y el error que sale al final ya no dice donde se empezo a buscar.
const maxModuleSearchDepth = 64

// FindModule busca el go.mod que gobierna start, subiendo por el arbol. Es todo
// lo que hace falta para el caso que interesa —analizar un modulo local— y evita
// depender de `golang.org/x/tools/go/packages`, que resuelve tambien el GOPATH y
// las dependencias descargadas. Si algun dia hay que analizar codigo fuera del
// modulo, ese es el momento de reevaluar la dependencia, no antes.
func FindModule(start string) (Module, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return Module{}, fmt.Errorf("resolve %s: %w", start, err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return Module{}, fmt.Errorf("resolve %s: %w", start, err)
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for depth := 0; depth < maxModuleSearchDepth; depth++ {
		candidate := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(candidate)
		if err == nil {
			path, err := modulePath(data)
			if err != nil {
				return Module{}, fmt.Errorf("%s: %w", candidate, err)
			}

			return Module{Path: path, Dir: dir}, nil
		}
		if !os.IsNotExist(err) {
			return Module{}, fmt.Errorf("read %s: %w", candidate, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return Module{}, fmt.Errorf("no go.mod found at or above %s", start)
}

// modulePath lee la directiva `module` de un go.mod. Se hace a mano en vez de con
// `golang.org/x/mod/modfile` porque es una sola linea y la dependencia costaria
// mas que el codigo que ahorra.
func modulePath(data []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		// Se compara la primera palabra completa, no el prefijo: `moduleFoo = 1`
		// empieza con "module" y no es la directiva.
		fields := strings.Fields(text)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}

		rest := strings.TrimSpace(strings.TrimPrefix(text, "module"))
		// Los comentarios de linea van despues de la ruta y no forman parte de ella.
		if idx := strings.Index(rest, "//"); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
		}
		// La forma entre comillas es valida y la emiten algunas herramientas.
		if unquoted, err := strconv.Unquote(rest); err == nil {
			rest = unquoted
		}
		if rest == "" {
			continue
		}

		return rest, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	return "", fmt.Errorf("no module directive found")
}

// SourcePath traduce una ruta del perfil a un archivo en disco. El perfil de Go
// nombra los archivos por import path (`github.com/org/app/internal/cli/root.go`),
// no por ruta relativa, asi que hay que quitarle el prefijo del modulo.
//
// Devuelve ok en false, sin error, cuando la ruta pertenece a otro modulo: pasa
// de verdad al concatenar perfiles, y no es un fallo del analisis sino un archivo
// del que no tenemos fuente. Quien llama decide si lo reporta o lo ignora.
func (m Module) SourcePath(profilePath string) (path string, ok bool) {
	clean := strings.ReplaceAll(profilePath, "\\", "/")
	if m.Path == "" || clean == "" {
		return "", false
	}

	switch {
	case clean == m.Path:
		return "", false
	case strings.HasPrefix(clean, m.Path+"/"):
		relative := strings.TrimPrefix(clean, m.Path+"/")

		return filepath.Join(m.Dir, filepath.FromSlash(relative)), true
	}

	return "", false
}
