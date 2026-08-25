package coverage

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// Parser lee un formato concreto de reporte. Detect mira el **contenido**, no la
// extension: un jacoco.xml renombrado a cobertura.xml se sigue detectando bien, y
// un coverage.out que no es de Go se rechaza en vez de parsearse a medias.
type Parser interface {
	Name() string
	Detect(data []byte) bool
	Parse(data []byte) (Report, error)
}

// parsers esta ordenado de mas especifico a menos: los tres XML comparten forma y
// el desempate lo hace cada Detect mirando el elemento raiz y su primer hijo.
func parsers() []Parser {
	return []Parser{
		goProfileParser{},
		lcovParser{},
		jacocoParser{},
		cloverParser{},
		coberturaParser{},
	}
}

// ParseFile lee el reporte de una ruta, infiriendo su formato del contenido.
func ParseFile(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read coverage report: %w", err)
	}

	report, err := Parse(data)
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}

	return report, nil
}

// Parse detecta el formato y delega. El error nombra los formatos intentados,
// porque "formato desconocido" a secas no le dice a nadie que esperaba el comando.
func Parse(data []byte) (Report, error) {
	available := parsers()
	for _, parser := range available {
		if !parser.Detect(data) {
			continue
		}

		report, err := parser.Parse(data)
		if err != nil {
			return Report{}, fmt.Errorf("parse %s coverage report: %w", parser.Name(), err)
		}
		report.Format = parser.Name()

		return report, nil
	}

	names := make([]string, 0, len(available))
	for _, parser := range available {
		names = append(names, parser.Name())
	}

	return Report{}, fmt.Errorf("unrecognized coverage report format; tried %s", strings.Join(names, ", "))
}

// SupportedFormats lista los formatos que se saben leer, para mensajes de ayuda.
func SupportedFormats() []string {
	available := parsers()
	names := make([]string, 0, len(available))
	for _, parser := range available {
		names = append(names, parser.Name())
	}

	return names
}

// xmlShape devuelve el nombre del elemento raiz y el de su primer hijo. Es lo
// unico que separa a Clover de Cobertura: los dos tienen raiz <coverage>, pero el
// primero cuelga <project> y el segundo <packages>.
func xmlShape(data []byte) (root string, child string, ok bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	for {
		token, err := decoder.Token()
		if err != nil {
			return root, "", root != ""
		}

		start, isStart := token.(xml.StartElement)
		if !isStart {
			continue
		}

		if root == "" {
			root = start.Name.Local
			continue
		}

		return root, start.Name.Local, true
	}
}

func decodeXML(data []byte, target any) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	// Los reportes reales traen encodings variados (Clover de PHPUnit sale en
	// UTF-8, pero herramientas de Java a veces emiten ISO-8859-1). Sin esto, el
	// decoder falla en vez de leer lo que si entiende.
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	return decoder.Decode(target)
}
