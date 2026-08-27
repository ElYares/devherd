package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// serviceFiles declara la configuracion de cada servicio. Redis y Mailpit no
// necesitan ninguna, y por eso el problema no se veia hasta ahora.
var serviceFiles = map[string][]ManagedFile{}

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
func ServiceFiles(service string) []ManagedFile {
	files := serviceFiles[service]
	out := make([]ManagedFile, len(files))
	copy(out, files)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	return out
}

// ensureServiceFiles escribe la configuracion de un servicio respetando lo que el
// usuario haya editado. Devuelve que paso con cada archivo para que el comando lo
// pueda decir: escribir configuracion en silencio dentro del directorio de alguien
// es como se pierde una tarde buscando por que un servicio no toma sus ajustes.
func (m Manager) ensureServiceFiles(service string, force bool) ([]FileResult, error) {
	files := ServiceFiles(service)
	if len(files) == 0 {
		return nil, nil
	}

	results := make([]FileResult, 0, len(files))
	for _, file := range files {
		result, err := m.ensureFile(file, force)
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
