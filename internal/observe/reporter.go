package observe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	observetemplates "github.com/devherd/devherd/templates/observe"
)

// reporters mapea cada stack con el archivo que hay que escribir dentro del
// proyecto. Solo estan los stacks cuyo reporter esta verificado contra un
// proyecto real: prometer soporte que no se ha probado sale mas caro que no
// ofrecerlo.
var reporters = map[string]reporterTemplate{
	"laravel": {
		Source: observetemplates.ReporterLaravel,
		Path:   filepath.Join("app", "Exceptions", "DevherdObserveReporter.php"),
		Wiring: "bootstrap/app.php: ->withExceptions(fn ($e) => $e->report(fn (Throwable $t) => DevherdObserveReporter::report($t)))",
	},
}

type reporterTemplate struct {
	Source string
	Path   string
	Wiring string
}

// ErrReporterExists indica que ya hay un reporter en el proyecto. Se distingue
// para que el llamante ofrezca --force en vez de tratarlo como fallo.
var ErrReporterExists = errors.New("reporter already exists")

type ReporterResult struct {
	Path   string
	Stack  string
	Wiring string
}

// SupportedReporterStacks lista los stacks con reporter disponible.
func SupportedReporterStacks() []string {
	stacks := make([]string, 0, len(reporters))
	for stack := range reporters {
		stacks = append(stacks, stack)
	}
	sort.Strings(stacks)

	return stacks
}

// EnsureReporter escribe el reporter del stack dentro del proyecto. Nunca pisa
// un archivo existente sin force: ese archivo es codigo del proyecto y puede
// estar editado a mano.
func EnsureReporter(projectRoot, stack string, force bool) (ReporterResult, error) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return ReporterResult{}, fmt.Errorf("project root is required")
	}

	stack = strings.ToLower(strings.TrimSpace(stack))
	template, ok := reporters[stack]
	if !ok {
		return ReporterResult{}, fmt.Errorf(
			"no reporter available for stack %q; supported stacks: %s",
			stack, strings.Join(SupportedReporterStacks(), ", "),
		)
	}

	payload, err := observetemplates.Files.ReadFile(template.Source)
	if err != nil {
		return ReporterResult{}, fmt.Errorf("read embedded reporter: %w", err)
	}

	path := filepath.Join(projectRoot, template.Path)
	result := ReporterResult{Path: path, Stack: stack, Wiring: template.Wiring}

	if _, err := os.Stat(path); err == nil && !force {
		return result, ErrReporterExists
	} else if err != nil && !os.IsNotExist(err) {
		return result, fmt.Errorf("inspect reporter path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return result, fmt.Errorf("create reporter directory: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return result, fmt.Errorf("write reporter: %w", err)
	}

	return result, nil
}
