// Package coverage lee reportes de cobertura de distintos ecosistemas y calcula
// sobre ellos. No instrumenta codigo ni mide nada: eso lo hace la herramienta
// nativa de cada stack (PHPUnit, JaCoCo, vitest, coverage.py, go test), y aqui se
// lee lo que dejaron.
package coverage

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Unit es lo que cuenta un reporte. No es un detalle cosmetico: Go mide
// sentencias mientras LCOV, JaCoCo y Cobertura miden lineas, asi que un 58% de un
// stack y un 58% de otro no significan lo mismo. Llevarla como dato de primera
// clase es lo que impide sumarlos por descuido.
type Unit string

const (
	UnitStatements Unit = "statements"
	UnitLines      Unit = "lines"
)

// Block es un bloque basico del reporte, con el rango de lineas que ocupa en el
// fuente. Solo lo produce el perfil de Go: es el unico de los cinco formatos que
// dice **donde** empieza y termina cada bloque medido, y sin ese dato no se puede
// atribuir cobertura a funciones. Los demas parsers dejan Blocks vacio, y esa
// ausencia es justo lo que distingue "hay analisis estructural disponible" de
// "solo hay agregacion por archivo".
type Block struct {
	StartLine int
	EndLine   int
	// Stmts son las unidades del bloque, en las unidades del reporte.
	Stmts int
	// Count es cuantas veces se ejecuto. Cubierto es Count > 0; el numero exacto
	// solo tiene sentido en modo `count` y se conserva porque no cuesta nada.
	Count int
}

// Covered dice si el bloque llego a ejecutarse.
func (b Block) Covered() bool {
	return b.Count > 0
}

// FileReport es la cobertura de un archivo, en las unidades del reporte.
type FileReport struct {
	Path    string `json:"path"`
	Total   int    `json:"total"`
	Covered int    `json:"covered"`
	// Blocks es el detalle por rango de lineas, cuando el formato lo trae. Vacio
	// no significa "sin cobertura", significa "este formato no lo reporta".
	//
	// Fuera del JSON a proposito: es el insumo del analisis estructural, no parte
	// del reporte. Serializarlo multiplica por 50 la salida de `coverage --json`
	// (22.214 lineas contra 400, medido sobre este repo) para dar un detalle que
	// nadie consume crudo. Lo que se publica es la atribucion por funcion, que es
	// lo que se pidio y ademas es legible.
	Blocks []Block `json:"-"`
}

// HasBlocks dice si el archivo trae detalle por rango de lineas, que es la
// precondicion de cualquier analisis estructural.
func (f FileReport) HasBlocks() bool {
	return len(f.Blocks) > 0
}

// Uncovered es la masa sin cubrir, que es lo que decide donde trabajar. Un
// archivo de 800 unidades al 40% deja 480 sin cubrir; uno de 3 al 0% deja 3.
// Ordenar por porcentaje pondria primero al segundo.
func (f FileReport) Uncovered() int {
	return f.Total - f.Covered
}

// Percent devuelve el porcentaje cubierto, o 0 si el archivo no tiene unidades.
func (f FileReport) Percent() float64 {
	return percent(f.Covered, f.Total)
}

// Report es un reporte de cobertura ya normalizado, sea cual sea el formato del
// que se leyo.
type Report struct {
	// Format es el parser que lo produjo: lcov, clover, jacoco, cobertura, go.
	Format string       `json:"format"`
	Unit   Unit         `json:"unit"`
	Files  []FileReport `json:"files"`
}

// Total suma las unidades de todos los archivos.
func (r Report) Total() int {
	total := 0
	for _, file := range r.Files {
		total += file.Total
	}

	return total
}

// Covered suma las unidades cubiertas de todos los archivos.
func (r Report) Covered() int {
	covered := 0
	for _, file := range r.Files {
		covered += file.Covered
	}

	return covered
}

// Uncovered es la masa total sin cubrir.
func (r Report) Uncovered() int {
	return r.Total() - r.Covered()
}

// Percent es el total **ponderado por unidades**, nunca el promedio de los
// porcentajes por archivo: un archivo de 3 lineas al 100% no vale lo mismo que
// uno de 800 al 40%, y promediarlos daria 70% cuando la realidad es 40,1%.
func (r Report) Percent() float64 {
	return percent(r.Covered(), r.Total())
}

// IsEmpty distingue "no hay datos de cobertura" de "nada esta cubierto". Los dos
// se ven como 0% y significan cosas opuestas.
func (r Report) IsEmpty() bool {
	return r.Total() == 0
}

// HasBlocks dice si el reporte trae detalle por rango de lineas en al menos un
// archivo. Es la precondicion del analisis estructural: sin bloques solo se puede
// agregar por archivo, y decirlo es mas honesto que inventar un techo.
func (r Report) HasBlocks() bool {
	for _, file := range r.Files {
		if file.HasBlocks() {
			return true
		}
	}

	return false
}

// ByUncovered devuelve los archivos ordenados por masa sin cubrir, de mayor a
// menor. El desempate es por ruta, para que la salida no dependa del orden en que
// el parser recorrio el archivo.
func (r Report) ByUncovered() []FileReport {
	files := append([]FileReport(nil), r.Files...)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Uncovered() != files[j].Uncovered() {
			return files[i].Uncovered() > files[j].Uncovered()
		}

		return files[i].Path < files[j].Path
	})

	return files
}

// ByPath devuelve los archivos ordenados por ruta.
func (r Report) ByPath() []FileReport {
	files := append([]FileReport(nil), r.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return files
}

// GroupReport es la cobertura agregada de un directorio o paquete.
type GroupReport struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Covered int    `json:"covered"`
	Files   int    `json:"files"`
}

// Uncovered es la masa sin cubrir del grupo.
func (g GroupReport) Uncovered() int {
	return g.Total - g.Covered
}

// Percent es el porcentaje del grupo, ponderado por unidades.
func (g GroupReport) Percent() float64 {
	return percent(g.Covered, g.Total)
}

// Groups agrega por directorio, que en la practica es el paquete o modulo, y lo
// devuelve **ordenado por masa sin cubrir** igual que ByUncovered. En orden
// alfabetico obliga a leer los 38 directorios de un proyecto real para encontrar
// donde esta el bulto, que es justo el trabajo que este comando viene a ahorrar.
func (r Report) Groups() []GroupReport {
	index := map[string]*GroupReport{}
	for _, file := range r.Files {
		name := path.Dir(strings.ReplaceAll(file.Path, "\\", "/"))
		if name == "" {
			name = "."
		}

		group, ok := index[name]
		if !ok {
			group = &GroupReport{Name: name}
			index[name] = group
		}
		group.Total += file.Total
		group.Covered += file.Covered
		group.Files++
	}

	groups := make([]GroupReport, 0, len(index))
	for _, group := range index {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Uncovered() != groups[j].Uncovered() {
			return groups[i].Uncovered() > groups[j].Uncovered()
		}

		return groups[i].Name < groups[j].Name
	})

	return groups
}

// Merge combina dos reportes del mismo formato de medicion. Falla si las unidades
// no coinciden: sumar sentencias de Go con lineas de LCOV daria un numero que no
// significa nada, y el error es silencioso si nadie lo impide aqui.
func (r Report) Merge(other Report) (Report, error) {
	switch {
	case r.IsEmpty() && len(r.Files) == 0:
		return other, nil
	case other.IsEmpty() && len(other.Files) == 0:
		return r, nil
	case r.Unit != other.Unit:
		return Report{}, fmt.Errorf(
			"cannot merge coverage reports measured in different units: %s (%s) and %s (%s)",
			r.Format, r.Unit, other.Format, other.Unit)
	}

	index := map[string]FileReport{}
	order := make([]string, 0, len(r.Files)+len(other.Files))
	for _, file := range append(append([]FileReport(nil), r.Files...), other.Files...) {
		existing, ok := index[file.Path]
		if !ok {
			index[file.Path] = file
			order = append(order, file.Path)
			continue
		}

		// El mismo archivo medido dos veces: se queda la medicion con mas
		// cobertura, no la suma, que inventaria unidades que no existen.
		if file.Covered > existing.Covered {
			existing.Covered = file.Covered
			// Los bloques siguen a la medicion que gana. Mezclarlos rango a rango
			// exigiria que las dos corridas vieran el mismo fuente, y nada lo
			// garantiza: quedarse con los del perdedor daria una atribucion por
			// funcion que no corresponde a los totales que se muestran.
			if file.HasBlocks() {
				existing.Blocks = file.Blocks
			}
		}
		if file.Total > existing.Total {
			existing.Total = file.Total
		}
		// Un formato con bloques y otro sin ellos: se conservan los que hay, que
		// es mejor que perderlos porque el otro lado no los reporta.
		if !existing.HasBlocks() && file.HasBlocks() {
			existing.Blocks = file.Blocks
		}
		index[file.Path] = existing
	}

	merged := Report{Format: r.Format, Unit: r.Unit, Files: make([]FileReport, 0, len(order))}
	if other.Format != r.Format {
		merged.Format = r.Format + "+" + other.Format
	}
	for _, key := range order {
		merged.Files = append(merged.Files, index[key])
	}

	return merged, nil
}

func percent(covered, total int) float64 {
	if total <= 0 {
		return 0
	}

	return float64(covered) / float64(total) * 100
}
