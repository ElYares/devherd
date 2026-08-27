package coverage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

// FunctionKind distingue las tres formas en que se declara codigo ejecutable en
// Go. La distincion no es cosmetica: un closure dentro de un constructor no se
// puede probar sin llamar al constructor, y esa es la diferencia que separa la
// cobertura alcanzable de la que exige un refactor.
type FunctionKind string

const (
	// KindFunc es una funcion de paquete, sin receptor.
	KindFunc FunctionKind = "func"
	// KindMethod es una funcion con receptor.
	KindMethod FunctionKind = "method"
	// KindClosure es una funcion literal, viva dentro de otra.
	KindClosure FunctionKind = "closure"
)

// FunctionReport es la cobertura atribuida a una funcion concreta. Es el dato que
// ninguna herramienta de los cinco ecosistemas entrega, y la razon de ser del
// analisis estructural.
type FunctionReport struct {
	// File es la ruta tal como la nombra el perfil, para poder cruzarla con el
	// reporte por archivo sin traducir dos veces.
	File string `json:"file"`
	// Name es el nombre calificado: `Store.Save` para un metodo,
	// `newCoverageCmd.RunE` para el closure asignado a ese campo.
	Name string       `json:"name"`
	Kind FunctionKind `json:"kind"`
	// Enclosing es la funcion que la contiene, vacia si es de paquete.
	Enclosing string `json:"enclosing,omitempty"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Total     int    `json:"total"`
	Covered   int    `json:"covered"`
}

// Uncovered es la masa sin cubrir de la funcion, que es lo que decide si vale la
// pena atacarla. Una funcion de 80 sentencias al 0% pesa mas que una de 3.
func (f FunctionReport) Uncovered() int {
	return f.Total - f.Covered
}

// Percent es el porcentaje cubierto de la funcion.
func (f FunctionReport) Percent() float64 {
	return percent(f.Covered, f.Total)
}

// FileAttribution es el resultado de atribuir un archivo, con lo que no se pudo
// atribuir contado aparte en vez de repartido a la fuerza.
type FileAttribution struct {
	File      string           `json:"file"`
	Functions []FunctionReport `json:"functions"`
	// OrphanStmts son las sentencias de bloques que no cayeron dentro de ninguna
	// funcion. Deberia ser cero; contarlas explicitamente es lo que convierte un
	// desajuste entre perfil y fuente en un numero visible en vez de en una
	// atribucion silenciosamente torcida.
	OrphanStmts int `json:"orphan_stmts,omitempty"`
	// OrphanCovered son las cubiertas de esas mismas sentencias.
	OrphanCovered int `json:"orphan_covered,omitempty"`
}

// AttributeFile reparte los bloques de un archivo entre las funciones que declara
// su fuente. El bloque pertenece a la funcion **mas interna** que lo contiene, no
// a la ultima que empieza antes: esa heuristica confunde un closure con el
// constructor que lo envuelve, que es exactamente la distincion que hace falta.
func AttributeFile(file FileReport, sourcePath string) (FileAttribution, error) {
	functions, err := parseFunctions(sourcePath)
	if err != nil {
		return FileAttribution{}, err
	}

	attribution := FileAttribution{File: file.Path}
	totals := make([]FunctionReport, len(functions))
	for i, fn := range functions {
		totals[i] = FunctionReport{
			File:      file.Path,
			Name:      fn.name,
			Kind:      fn.kind,
			Enclosing: fn.enclosing,
			StartLine: fn.start,
			EndLine:   fn.end,
		}
	}

	for _, block := range file.Blocks {
		index := innermostContaining(functions, block)
		if index < 0 {
			attribution.OrphanStmts += block.Stmts
			if block.Covered() {
				attribution.OrphanCovered += block.Stmts
			}

			continue
		}

		totals[index].Total += block.Stmts
		if block.Covered() {
			totals[index].Covered += block.Stmts
		}
	}

	// Las funciones sin una sola sentencia medida no se reportan: son las que el
	// perfil no menciona —interfaces, stubs vacios, codigo tras build tags que no
	// se compilo— y listarlas al 0% las haria parecer abandono medido.
	attribution.Functions = make([]FunctionReport, 0, len(totals))
	for _, fn := range totals {
		if fn.Total == 0 {
			continue
		}
		attribution.Functions = append(attribution.Functions, fn)
	}

	return attribution, nil
}

// goFunc es una funcion encontrada en el AST, con el rango de lineas que ocupa.
type goFunc struct {
	name      string
	enclosing string
	kind      FunctionKind
	start     int
	end       int
}

// innermostContaining devuelve el indice de la funcion mas ajustada que contiene
// al bloque, o -1 si ninguna lo hace. Se ancla en la linea de inicio del bloque:
// el perfil de Go nunca parte un bloque entre dos funciones, y usar el rango
// completo obligaria a decidir que hacer con un solapamiento que no ocurre.
func innermostContaining(functions []goFunc, block Block) int {
	best := -1
	for i, fn := range functions {
		if block.StartLine < fn.start || block.StartLine > fn.end {
			continue
		}
		if best < 0 {
			best = i

			continue
		}
		// Mas interna es la que empieza despues; a igual inicio, la que termina
		// antes. Un closure en la primera linea del cuerpo de su funcion empata en
		// inicio y solo el fin lo desempata.
		switch {
		case fn.start > functions[best].start:
			best = i
		case fn.start == functions[best].start && fn.end < functions[best].end:
			best = i
		}
	}

	return best
}

// parseFunctions lee un archivo Go y devuelve sus funciones y closures con sus
// rangos de lineas. Se usa `go/parser` y no la heuristica de `^func` que se aplico
// a mano: aquella no ve los closures, y confunde con una declaracion cualquier
// comentario o cadena que empiece con esa palabra.
func parseFunctions(path string) ([]goFunc, error) {
	fset := token.NewFileSet()
	// Sin comentarios: no aportan al rango de las funciones y hacen el arbol mas
	// pesado en archivos grandes.
	parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	lineOf := func(pos token.Pos) int {
		return fset.Position(pos).Line
	}

	labels := litLabels(parsed)
	found := make([]goFunc, 0, 32)
	// scope lleva el nombre de la funcion que se esta recorriendo, para nombrar los
	// closures que cuelgan de ella. counters cuenta los closures anonimos por
	// funcion, que es como los nombra tambien `go tool cover -func`.
	var scope []string
	counters := map[string]int{}

	var walk func(node ast.Node)
	walk = func(node ast.Node) {
		ast.Inspect(node, func(n ast.Node) bool {
			if n == node {
				return true
			}

			switch fn := n.(type) {
			case *ast.FuncDecl:
				name, kind := declName(fn)
				found = append(found, goFunc{
					name:  name,
					kind:  kind,
					start: lineOf(fn.Pos()),
					end:   lineOf(fn.End()),
				})
				if fn.Body == nil {
					return false
				}
				scope = append(scope, name)
				walk(fn.Body)
				scope = scope[:len(scope)-1]

				return false

			case *ast.FuncLit:
				enclosing := ""
				if len(scope) > 0 {
					enclosing = scope[len(scope)-1]
				}
				name := closureName(enclosing, labels[fn], counters)
				found = append(found, goFunc{
					name:      name,
					enclosing: enclosing,
					kind:      KindClosure,
					start:     lineOf(fn.Pos()),
					end:       lineOf(fn.End()),
				})
				scope = append(scope, name)
				walk(fn.Body)
				scope = scope[:len(scope)-1]

				return false
			}

			return true
		})
	}
	walk(parsed)

	sort.Slice(found, func(i, j int) bool {
		if found[i].start != found[j].start {
			return found[i].start < found[j].start
		}

		return found[i].end < found[j].end
	})

	return found, nil
}

// declName arma el nombre calificado de una declaracion y dice si es metodo.
func declName(fn *ast.FuncDecl) (string, FunctionKind) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name, KindFunc
	}

	return receiverType(fn.Recv.List[0].Type) + "." + fn.Name.Name, KindMethod
}

// receiverType saca el nombre del tipo receptor, sin punteros ni parametros de
// tipo, que no aportan a la identificacion y ensucian la tabla.
func receiverType(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverType(typed.X)
	case *ast.IndexExpr:
		return receiverType(typed.X)
	case *ast.IndexListExpr:
		return receiverType(typed.X)
	case *ast.Ident:
		return typed.Name
	}

	return "?"
}

// litLabels recorre el archivo una sola vez y anota, para cada funcion literal,
// el nombre bajo el que se declaro. `RunE` dice mucho mas que `func1`, y es justo
// el caso que importa: la masa de `internal/cli` vive en closures asignados a
// campos de cobra. Se hace en una pasada previa y no buscando cada literal en el
// subarbol de su funcion, que seria cuadratico en archivos grandes.
func litLabels(file *ast.File) map[*ast.FuncLit]string {
	labels := map[*ast.FuncLit]string{}

	note := func(expr ast.Expr, name string) {
		lit, ok := expr.(*ast.FuncLit)
		if !ok || name == "" || name == "_" {
			return
		}
		labels[lit] = name
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		// `RunE: func(...) {...}` dentro de un literal de struct.
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok {
				note(node.Value, key.Name)
			}

		// `handler := func(...) {...}`
		case *ast.AssignStmt:
			for i, value := range node.Rhs {
				if i >= len(node.Lhs) {
					break
				}
				if name, ok := node.Lhs[i].(*ast.Ident); ok {
					note(value, name.Name)
				}
			}

		// `var handler = func(...) {...}`
		case *ast.ValueSpec:
			for i, value := range node.Values {
				if i >= len(node.Names) {
					break
				}
				note(value, node.Names[i].Name)
			}
		}

		return true
	})

	return labels
}

// closureName califica el closure con la funcion que lo contiene. Sin etiqueta se
// numera por orden de aparicion dentro de esa funcion, no del archivo, para que
// agregar un closure en otra funcion no renumere los ajenos entre dos corridas.
func closureName(enclosing, label string, counters map[string]int) string {
	prefix := enclosing
	if prefix == "" {
		prefix = "init"
	}

	if label != "" {
		return prefix + "." + label
	}

	counters[prefix]++

	return prefix + ".func" + strconv.Itoa(counters[prefix])
}
