package coverage

import (
	"bufio"
	"bytes"
	"fmt"
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

	blocks := map[blockKey]int{}
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

		key := blockKey{file: location, span: span, stmts: stmts}
		// La presencia se comprueba con el segundo valor del mapa, nunca contra su
		// cero: un bloque sin cubrir tiene count 0, y compararlo con `> blocks[key]`
		// lo dejaria sin guardar. Entonces cada repeticion volveria a verse como
		// nueva y sus sentencias se contarian dos y tres veces.
		existing, seen := blocks[key]
		if !seen {
			order = append(order, key)
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
	}

	report := Report{Unit: UnitStatements, Files: make([]FileReport, 0, len(files))}
	for _, name := range files {
		report.Files = append(report.Files, *totals[name])
	}

	return report, nil
}
