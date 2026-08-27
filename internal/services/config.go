package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	servicestemplates "github.com/devherd/devherd/templates/services"
)

// ManagedFile es un archivo de configuracion que DevHerd escribe dentro del stack
// compartido para que un servicio funcione.
//
// La linea entre esto y el compose es deliberada: **el compose es de DevHerd, la
// configuracion es del usuario.** El compose es el catalogo de lo que DevHerd
// ofrece y se regenera siempre, porque si quedara congelado con la edicion de
// alguien, una version nueva del binario no podria ofrecer un servicio nuevo. Los
// archivos de configuracion son el ajuste de cada quien y se respetan.
type ManagedFile struct {
	// Path es la ruta relativa al directorio del stack compartido.
	Path string
	// Content es la plantilla que DevHerd escribe la primera vez.
	Content string
}

// ServiceOptions son los datos del entorno que necesitan las plantillas.
//
// HU-011 dejo las plantillas parametrizables fuera de alcance a proposito, y
// Prometheus es el caso que obliga a tenerlas: su unico contenido util es una
// direccion que solo se conoce en tiempo de ejecucion.
type ServiceOptions struct {
	// CollectorAddr es la direccion del collector de Observe **alcanzable desde un
	// contenedor**. No es 127.0.0.1: desde dentro de un contenedor, loopback es el
	// propio contenedor, y apuntar ahi deja un target caido sin explicacion.
	CollectorAddr string
}

// serviceFiles declara la configuracion de cada servicio. Redis y Mailpit no
// necesitan ninguna, y por eso el problema de las escrituras no se veia hasta que
// llego Prometheus.
var serviceFiles = map[string]func(ServiceOptions) ([]ManagedFile, error){
	"prometheus": prometheusFiles,
}

// prometheusFiles arma el prometheus.yml con el collector ya apuntado. Escribirlo
// a mano exige saber que la direccion no es loopback, cual es el gateway de la red
// compartida y donde va el archivo: justo el trabajo que DevHerd existe para
// ahorrar, y el unico motivo por el que empaquetar Prometheus tiene sentido.
func prometheusFiles(opts ServiceOptions) ([]ManagedFile, error) {
	addr := strings.TrimSpace(opts.CollectorAddr)
	if addr == "" {
		return nil, fmt.Errorf("the Observe collector address is required to configure Prometheus")
	}

	tmpl, err := template.New("prometheus.yml").Parse(servicestemplates.PrometheusConfig)
	if err != nil {
		return nil, fmt.Errorf("parse the Prometheus configuration template: %w", err)
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, struct{ CollectorAddr string }{CollectorAddr: addr}); err != nil {
		return nil, fmt.Errorf("render the Prometheus configuration: %w", err)
	}

	return []ManagedFile{{Path: "prometheus/prometheus.yml", Content: rendered.String()}}, nil
}

// FileState dice que se hizo con un archivo administrado, para poder contarlo.
type FileState int

const (
	// FileWritten es un archivo que no existia y se creo.
	FileWritten FileState = iota
	// FileUnchanged es un archivo que ya estaba y coincide con la plantilla.
	FileUnchanged
	// FileKept es un archivo que el usuario edito y se dejo como estaba.
	FileKept
	// FileRestored es un archivo editado que se devolvio a la plantilla con force.
	FileRestored
)

// FileResult es lo que paso con un archivo administrado.
type FileResult struct {
	Path  string
	State FileState
	// Backup es la ruta de la copia, cuando se restauro una plantilla sobre una
	// edicion del usuario.
	Backup string
}

// backupSuffix es lo que se le agrega al archivo antes de pisarlo con --force.
// Restaurar sin copia convierte una bandera en una perdida de trabajo.
const backupSuffix = ".bak"

// ServiceFiles devuelve la configuracion declarada por un servicio, o nada.
func ServiceFiles(service string, opts ServiceOptions) ([]ManagedFile, error) {
	build, ok := serviceFiles[service]
	if !ok {
		return nil, nil
	}

	files, err := build(opts)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return files, nil
}

// NeedsCollector dice si un servicio necesita saber donde escucha el collector.
// Se consulta **antes** de arrancar nada, para poder avisar cuando la direccion no
// sirve en vez de dejar un target caido que se descubre media hora despues.
func NeedsCollector(service string) bool {
	return service == "prometheus"
}

// ensureServiceFiles escribe la configuracion de un servicio respetando lo que el
// usuario haya editado. Devuelve que paso con cada archivo para que el comando lo
// pueda decir: escribir configuracion en silencio dentro del directorio de alguien
// es como se pierde una tarde buscando por que un servicio no toma sus ajustes.
func (m Manager) ensureServiceFiles(service string, opts StartOptions) ([]FileResult, error) {
	files, err := ServiceFiles(service, ServiceOptions{CollectorAddr: opts.CollectorAddr})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	results := make([]FileResult, 0, len(files))
	for _, file := range files {
		result, err := m.ensureFile(file, opts.Force)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (m Manager) ensureFile(file ManagedFile, force bool) (FileResult, error) {
	path := filepath.Join(m.dir, filepath.FromSlash(file.Path))
	result := FileResult{Path: path}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileResult{}, fmt.Errorf("create directory for %s: %w", file.Path, err)
	}

	existing, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return FileResult{}, fmt.Errorf("write %s: %w", file.Path, err)
		}
		result.State = FileWritten

		return result, nil

	case err != nil:
		return FileResult{}, fmt.Errorf("read %s: %w", file.Path, err)
	}

	if string(existing) == file.Content {
		result.State = FileUnchanged

		return result, nil
	}

	if !force {
		// La edicion del usuario manda. Volver a escribir la plantilla aqui es
		// exactamente el bug que esta historia viene a arreglar.
		result.State = FileKept

		return result, nil
	}

	backup := path + backupSuffix
	if err := os.WriteFile(backup, existing, 0o644); err != nil {
		return FileResult{}, fmt.Errorf("back up %s: %w", file.Path, err)
	}
	if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
		return FileResult{}, fmt.Errorf("restore %s: %w", file.Path, err)
	}
	result.State = FileRestored
	result.Backup = backup

	return result, nil
}

// DescribeFileResults arma el aviso para quien corre el comando. Devuelve vacio
// cuando no hay nada que decir: un aviso que sale siempre se aprende a ignorar.
func DescribeFileResults(results []FileResult) string {
	var lines []string
	for _, result := range results {
		switch result.State {
		case FileKept:
			lines = append(lines,
				fmt.Sprintf("keeping your edited %s; it differs from the DevHerd template "+
					"(--force restores it, keeping a %s copy)", result.Path, backupSuffix))
		case FileRestored:
			lines = append(lines,
				fmt.Sprintf("restored %s from the DevHerd template; your version is at %s",
					result.Path, result.Backup))
		case FileWritten:
			lines = append(lines, fmt.Sprintf("wrote %s", result.Path))
		case FileUnchanged:
		}
	}

	return strings.Join(lines, "\n")
}
