package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Candidate es un reporte encontrado en el proyecto, con el porque de su lugar en
// la lista. La razon viaja con el candidato porque el comando tiene que **decir**
// que archivo uso: elegir en silencio entre dos reportes es como se cuela una
// medicion vieja sin que nadie lo note.
type Candidate struct {
	// Path es la ruta absoluta del reporte.
	Path string `json:"path"`
	// RelPath es la ruta relativa a la raiz del proyecto, que es la que se muestra.
	RelPath string `json:"rel_path"`
	// Stack es el stack cuya convencion coloco este archivo aqui, vacio si se
	// encontro por una convencion compartida entre varios.
	Stack string `json:"stack,omitempty"`
	// Managed marca los reportes que escribio DevHerd con `--run`. Se distinguen
	// porque pueden ser de una corrida vieja, y el del proyecto manda sobre ellos.
	Managed bool `json:"managed,omitempty"`
}

// stackReportPaths son las rutas donde cada ecosistema deja su reporte por
// convencion. El orden dentro de cada stack importa: la primera que exista gana.
//
// Solo estan los stacks que el detector sabe reconocer. JaCoCo se lee sin
// problema con --report, pero no se puede descubrir: el detector no distingue un
// proyecto Java, y adivinar por la presencia de un pom.xml seria detectar stacks
// desde el paquete equivocado.
func stackReportPaths() map[string][]string {
	return map[string][]string{
		"laravel": {
			"coverage/clover.xml",
			"build/logs/clover.xml",
			"clover.xml",
		},
		"go": {
			"coverage.out",
			"cover.out",
			"coverage.txt",
		},
		"vue": {
			"coverage/lcov.info",
			"coverage/clover.xml",
		},
		"node": {
			"coverage/lcov.info",
			"coverage/clover.xml",
		},
		"python": {
			"coverage.xml",
			"htmlcov/coverage.xml",
		},
		"flask": {
			"coverage.xml",
			"htmlcov/coverage.xml",
		},
		"vue+flask": {
			"coverage/lcov.info",
			"coverage.xml",
		},
	}
}

// DiscoverableStacks lista los stacks cuyo reporte se sabe encontrar solo.
func DiscoverableStacks() []string {
	paths := stackReportPaths()
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// managedReportNames son los reportes que deja `--run`, en el orden en que se
// buscan. Se generan desde los mismos perfiles que los escriben, para que agregar
// un stack a `--run` no obligue a acordarse de tocar esta lista tambien.
func managedReportNames() []string {
	seen := map[string]bool{}
	names := make([]string, 0, 4)
	for _, profile := range stackProfiles() {
		name := ManagedReportPrefix + profile.reportExt
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// Discovery es el resultado de buscar un reporte en un proyecto.
type Discovery struct {
	// Chosen es el reporte que se va a leer.
	Chosen Candidate `json:"chosen"`
	// Others son los demas reportes encontrados. Se listan en vez de descartarse
	// en silencio: si hay dos, quien corre el comando tiene que enterarse.
	Others []Candidate `json:"others,omitempty"`
	// Searched son las rutas donde se busco, para que un "no encontrado" diga
	// donde se miro en vez de dejar al usuario adivinando.
	Searched []string `json:"searched"`
}

// ErrNoReportFound se devuelve cuando no hay ningun reporte. Lleva las rutas que
// se miraron y el comando que generaria uno, porque "no encontrado" a secas deja
// al usuario sin siguiente paso.
type ErrNoReportFound struct {
	Stack    string
	Root     string
	Searched []string
}

func (e ErrNoReportFound) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "no coverage report found in %s", e.Root)
	if e.Stack != "" {
		fmt.Fprintf(&b, " for stack %q", e.Stack)
	}
	b.WriteString("\n\nlooked for:")
	for _, path := range e.Searched {
		fmt.Fprintf(&b, "\n  %s", path)
	}
	if hint := generateHint(e.Stack); hint != "" {
		fmt.Fprintf(&b, "\n\ngenerate one with:\n  %s", hint)
		fmt.Fprintf(&b, "\n\nor let DevHerd do it:\n  devherd coverage --run")
	}

	return b.String()
}

// generateHint es el comando que produce un reporte para ese stack. Es lo que
// convierte el error en un siguiente paso.
func generateHint(stack string) string {
	switch strings.ToLower(strings.TrimSpace(stack)) {
	case "laravel":
		return "php artisan test --coverage-clover coverage/clover.xml"
	case "go":
		return "go test ./... -coverprofile=coverage.out"
	case "vue", "node":
		return "npx vitest run --coverage"
	case "python", "flask":
		return "coverage run -m pytest && coverage xml"
	case "vue+flask":
		return "npx vitest run --coverage   (front)\n  coverage run -m pytest && coverage xml   (back)"
	}

	return ""
}

// DiscoverReport busca el reporte de cobertura de un proyecto por la convencion
// de su stack. Devuelve tambien lo que no eligio y donde miro.
//
// El reporte del proyecto gana sobre el que escribe `--run`: el administrado
// puede ser de una corrida vieja, y preferirlo en silencio es como se lee una
// medicion caduca creyendo que es la de hoy.
func DiscoverReport(root, stack string) (Discovery, error) {
	if strings.TrimSpace(root) == "" {
		return Discovery{}, fmt.Errorf("project root is required")
	}

	stack = strings.ToLower(strings.TrimSpace(stack))
	catalog := stackReportPaths()
	discovery := Discovery{}
	seen := map[string]bool{}

	// stat mira una ruta y devuelve el candidato si hay un reporte usable ahi.
	stat := func(relative, forStack string, managed bool) (Candidate, bool) {
		if seen[relative] {
			return Candidate{}, false
		}
		seen[relative] = true

		absolute := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(absolute)
		if err != nil || info.IsDir() {
			return Candidate{}, false
		}
		// Un reporte de cero bytes no es un reporte: lo dejan las herramientas que
		// crearon el archivo y murieron antes de escribirlo, y elegirlo daria un
		// "formato desconocido" que manda a buscar el problema al lugar equivocado.
		if info.Size() == 0 {
			return Candidate{}, false
		}

		return Candidate{Path: absolute, RelPath: relative, Stack: forStack, Managed: managed}, true
	}

	// Primero las convenciones del stack detectado, que son las unicas elegibles.
	// Si el stack no se reconoce, lo son todas: es preferible encontrar algo y
	// decir de donde salio, a exigir --report por no saber nombrar el proyecto.
	primary := catalog[stack]
	primaryStack := stack
	if len(primary) == 0 {
		primary = allReportPaths(catalog)
		primaryStack = ""
	}

	eligible := make([]Candidate, 0, 4)
	for _, relative := range primary {
		discovery.Searched = append(discovery.Searched, relative)
		forStack := primaryStack
		if forStack == "" {
			forStack = stackOwning(catalog, relative)
		}
		if candidate, ok := stat(relative, forStack, false); ok {
			eligible = append(eligible, candidate)
		}
	}

	// Los reportes que deja --run son elegibles, pero detras del propio proyecto:
	// pueden ser de una corrida vieja, y preferirlos en silencio es como se lee una
	// medicion caduca creyendo que es la de hoy.
	for _, name := range managedReportNames() {
		discovery.Searched = append(discovery.Searched, name)
		if candidate, ok := stat(name, "", true); ok {
			eligible = append(eligible, candidate)
		}
	}

	if len(eligible) == 0 {
		return Discovery{}, ErrNoReportFound{Stack: stack, Root: root, Searched: discovery.Searched}
	}

	discovery.Chosen = eligible[0]
	discovery.Others = eligible[1:]

	// Las convenciones de los otros stacks no compiten por ser elegidas, pero si
	// aparecen se nombran. Un monorepo con front y back deja dos reportes de
	// formatos distintos, y tomar uno sin decir que existe el otro es el falso
	// positivo que este descubrimiento tiene que evitar.
	for _, relative := range allReportPaths(catalog) {
		if candidate, ok := stat(relative, stackOwning(catalog, relative), false); ok {
			discovery.Others = append(discovery.Others, candidate)
		}
	}

	return discovery, nil
}

// allReportPaths es la union de las convenciones de todos los stacks, en orden
// fijo para que dos corridas sobre el mismo proyecto elijan lo mismo.
func allReportPaths(catalog map[string][]string) []string {
	seen := map[string]bool{}
	paths := make([]string, 0, 16)
	for _, stack := range DiscoverableStacks() {
		for _, relative := range catalog[stack] {
			if seen[relative] {
				continue
			}
			seen[relative] = true
			paths = append(paths, relative)
		}
	}

	return paths
}

// stackOwning nombra el primer stack que declara esa ruta como suya, para poder
// decir de donde salio un reporte que se encontro sin saber el stack.
func stackOwning(catalog map[string][]string, relative string) string {
	for _, stack := range DiscoverableStacks() {
		for _, candidate := range catalog[stack] {
			if candidate == relative {
				return stack
			}
		}
	}

	return ""
}
