package coverage

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// lcovParser lee LCOV, que es lo que emiten vitest, jest, c8 e istanbul. Es el
// formato de Vue, React y TypeScript: los tres son el mismo ecosistema aqui.
//
//	SF:<ruta del archivo>
//	DA:<linea>,<ejecuciones>[,<checksum>]
//	LF:<lineas instrumentadas>
//	LH:<lineas ejecutadas>
//	end_of_record
type lcovParser struct{}

func (lcovParser) Name() string { return "lcov" }

func (lcovParser) Detect(data []byte) bool {
	// SF: abre cada registro y no aparece en ningun otro formato. Se busca al
	// inicio de linea para no confundirlo con texto dentro de otro documento.
	if bytes.HasPrefix(data, []byte("SF:")) {
		return true
	}

	return bytes.Contains(data, []byte("\nSF:")) || bytes.Contains(data, []byte("\r\nSF:"))
}

func (lcovParser) Parse(data []byte) (Report, error) {
	type record struct {
		// lines guarda las ejecuciones por numero de linea. Un mismo archivo puede
		// venir en varios registros (una suite por archivo de prueba) y hay que
		// fusionarlos por linea, no sumar sus totales.
		lines map[int]int
		// declared son LF/LH, que se usan solo cuando el reporte no trae DA.
		declaredTotal   int
		declaredCovered int
		hasDA           bool
	}

	records := map[string]*record{}
	order := make([]string, 0, 32)
	current := ""

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		switch {
		case strings.HasPrefix(text, "SF:"):
			current = strings.TrimSpace(strings.TrimPrefix(text, "SF:"))
			if _, ok := records[current]; !ok {
				records[current] = &record{lines: map[int]int{}}
				order = append(order, current)
			}

		case text == "end_of_record":
			current = ""

		case strings.HasPrefix(text, "DA:") && current != "":
			value := strings.TrimPrefix(text, "DA:")
			parts := strings.Split(value, ",")
			if len(parts) < 2 {
				return Report{}, fmt.Errorf("line %d: malformed DA entry %q", line, text)
			}
			number, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return Report{}, fmt.Errorf("line %d: DA line number: %w", line, err)
			}
			hits, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return Report{}, fmt.Errorf("line %d: DA hit count: %w", line, err)
			}

			entry := records[current]
			entry.hasDA = true
			// Hay que insertar aunque hits sea 0: una linea sin cubrir es un dato,
			// y compararla contra el cero del mapa vacio la haria desaparecer.
			if existing, seen := entry.lines[number]; !seen || hits > existing {
				entry.lines[number] = hits
			}

		case strings.HasPrefix(text, "LF:") && current != "":
			value, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(text, "LF:")))
			if err != nil {
				return Report{}, fmt.Errorf("line %d: LF value: %w", line, err)
			}
			if value > records[current].declaredTotal {
				records[current].declaredTotal = value
			}

		case strings.HasPrefix(text, "LH:") && current != "":
			value, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(text, "LH:")))
			if err != nil {
				return Report{}, fmt.Errorf("line %d: LH value: %w", line, err)
			}
			if value > records[current].declaredCovered {
				records[current].declaredCovered = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("read lcov report: %w", err)
	}

	report := Report{Unit: UnitLines, Files: make([]FileReport, 0, len(order))}
	for _, name := range order {
		entry := records[name]
		file := FileReport{Path: name}

		if entry.hasDA {
			for _, hits := range entry.lines {
				file.Total++
				if hits > 0 {
					file.Covered++
				}
			}
		} else {
			// Algunas herramientas emiten solo el resumen LF/LH. Es menos preciso
			// pero es lo unico que hay, y descartarlo seria peor.
			file.Total = entry.declaredTotal
			file.Covered = entry.declaredCovered
		}

		report.Files = append(report.Files, file)
	}

	return report, nil
}
