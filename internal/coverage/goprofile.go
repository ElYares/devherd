package coverage

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// goProfileParser lee el perfil de `go test -coverprofile`. Formato:
//
//	mode: set
//	ruta/archivo.go:12.20,14.3 2 1
//
// Es decir `archivo:iniLinea.iniCol,finLinea.finCol numSentencias vecesEjecutado`.
// Es el unico de los cinco formatos que mide **sentencias** y no lineas.
type goProfileParser struct{}

func (goProfileParser) Name() string { return "go" }

func (goProfileParser) Detect(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("mode:"))
}

func (goProfileParser) Parse(data []byte) (Report, error) {
	// Un mismo bloque puede aparecer repetido cuando se concatenan perfiles de
	// varios paquetes o corridas. `go tool cover` los fusiona en vez de sumarlos,
	// y hay que hacer lo mismo o el total sale inflado y deja de coincidir.
	type blockKey struct {
		file  string
		span  string
		stmts int
	}

	// Las lineas se guardan aparte de la clave: la clave desempata por el span
	// textual, que es exacto y barato, y esto es su traduccion a numeros para no
	// volver a parsearla al construir los bloques.
	type blockSpan struct {
		start int
		end   int
	}

	blocks := map[blockKey]int{}
	spans := map[blockKey]blockSpan{}
	order := make([]blockKey, 0, 256)
	files := make([]string, 0, 32)
	seenFile := map[string]bool{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "mode:") {
			continue
		}

		fields := strings.Fields(text)
		if len(fields) != 3 {
			return Report{}, fmt.Errorf("line %d: expected 3 fields, got %d", line, len(fields))
		}

		location, span, found := strings.Cut(fields[0], ":")
		if !found {
			return Report{}, fmt.Errorf("line %d: missing file separator in %q", line, fields[0])
		}

		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return Report{}, fmt.Errorf("line %d: statement count: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return Report{}, fmt.Errorf("line %d: execution count: %w", line, err)
		}

		startLine, endLine, err := spanLines(span)
		if err != nil {
			return Report{}, fmt.Errorf("line %d: %w", line, err)
		}

		key := blockKey{file: location, span: span, stmts: stmts}
		// La presencia se comprueba con el segundo valor del mapa, nunca contra su
		// cero: un bloque sin cubrir tiene count 0, y compararlo con `> blocks[key]`
		// lo dejaria sin guardar. Entonces cada repeticion volveria a verse como
		// nueva y sus sentencias se contarian dos y tres veces.
		existing, seen := blocks[key]
		if !seen {
			order = append(order, key)
			spans[key] = blockSpan{start: startLine, end: endLine}
		}
		// Fusionar quedandose con el mayor: en modo `count` el mismo bloque puede
		// venir con distintas ejecuciones, y lo que importa es si se ejecuto.
		if !seen || count > existing {
			blocks[key] = count
		}

		if !seenFile[location] {
			seenFile[location] = true
			files = append(files, location)
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("read go coverage profile: %w", err)
	}

	totals := map[string]*FileReport{}
	for _, key := range order {
		file, ok := totals[key.file]
		if !ok {
			file = &FileReport{Path: key.file}
			totals[key.file] = file
		}

		file.Total += key.stmts
		if blocks[key] > 0 {
			file.Covered += key.stmts
		}
		file.Blocks = append(file.Blocks, Block{
			StartLine: spans[key].start,
			EndLine:   spans[key].end,
			Stmts:     key.stmts,
			Count:     blocks[key],
		})
	}

	report := Report{Unit: UnitStatements, Files: make([]FileReport, 0, len(files))}
	for _, name := range files {
		file := totals[name]
		// Los bloques se ordenan por linea, no por orden de aparicion: un perfil
		// concatenado de varias corridas los trae intercalados, y la atribucion a
		// funciones y la salida JSON tienen que dar lo mismo en las dos formas.
		sort.Slice(file.Blocks, func(i, j int) bool {
			if file.Blocks[i].StartLine != file.Blocks[j].StartLine {
				return file.Blocks[i].StartLine < file.Blocks[j].StartLine
			}

			return file.Blocks[i].EndLine < file.Blocks[j].EndLine
		})
		report.Files = append(report.Files, *file)
	}

	return report, nil
}

// spanLines saca las lineas de inicio y fin de un span `iniLinea.iniCol,finLinea.finCol`.
// Las columnas se descartan a proposito: la atribucion a funciones se hace por
// linea, y arrastrar columnas invitaria a comparaciones que el AST resuelve mejor.
func spanLines(span string) (start int, end int, err error) {
	from, to, found := strings.Cut(span, ",")
	if !found {
		return 0, 0, fmt.Errorf("malformed block span %q: expected start,end", span)
	}

	start, err = lineOf(from)
	if err != nil {
		return 0, 0, fmt.Errorf("block span %q: %w", span, err)
	}
	end, err = lineOf(to)
	if err != nil {
		return 0, 0, fmt.Errorf("block span %q: %w", span, err)
	}
	// Un bloque que termina antes de empezar no es un detalle cosmetico: rompe la
	// atribucion por funcion, que asume rangos bien formados.
	if end < start {
		return 0, 0, fmt.Errorf("block span %q ends before it starts", span)
	}

	return start, end, nil
}

// lineOf lee la parte `linea.columna` de un extremo del span.
func lineOf(position string) (int, error) {
	text, _, found := strings.Cut(position, ".")
	if !found {
		return 0, fmt.Errorf("malformed position %q: expected line.column", position)
	}

	line, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("position %q: %w", position, err)
	}
	if line <= 0 {
		return 0, fmt.Errorf("position %q: line numbers start at 1", position)
	}

	return line, nil
}
