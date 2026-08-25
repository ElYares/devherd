package coverage

import (
	"path"
	"strings"
)

// coberturaParser lee Cobertura, que es lo que emite coverage.py con
// `--cov-report=xml`. Es el formato de Python.
//
//	<coverage><packages><package><classes>
//	  <class filename="foo/bar.py"><lines><line number="1" hits="1"/>
//
// Comparte raiz <coverage> con Clover; el desempate es el primer hijo.
type coberturaParser struct{}

type coberturaDocument struct {
	Packages []coberturaPackage `xml:"packages>package"`
}

type coberturaPackage struct {
	Name    string             `xml:"name,attr"`
	Classes []coberturaClass   `xml:"classes>class"`
	Nested  []coberturaPackage `xml:"packages>package"`
}

type coberturaClass struct {
	Name     string          `xml:"name,attr"`
	Filename string          `xml:"filename,attr"`
	Lines    []coberturaLine `xml:"lines>line"`
}

type coberturaLine struct {
	Number int `xml:"number,attr"`
	Hits   int `xml:"hits,attr"`
}

func (coberturaParser) Name() string { return "cobertura" }

func (coberturaParser) Detect(data []byte) bool {
	root, child, ok := xmlShape(data)
	if !ok || root != "coverage" {
		return false
	}

	// <sources> suele preceder a <packages>; los dos descartan a Clover, que
	// cuelga <project>.
	return child == "packages" || child == "sources"
}

func (coberturaParser) Parse(data []byte) (Report, error) {
	var document coberturaDocument
	if err := decodeXML(data, &document); err != nil {
		return Report{}, err
	}

	// Un mismo archivo puede venir partido en varias <class> (coverage.py emite
	// una por clase de Python), asi que se fusiona por nombre de archivo y por
	// numero de linea. Sumar las clases contaria lineas repetidas.
	files := map[string]map[int]int{}
	order := make([]string, 0, 32)

	var walk func(pkgs []coberturaPackage)
	walk = func(pkgs []coberturaPackage) {
		for _, pkg := range pkgs {
			for _, class := range pkg.Classes {
				name := strings.TrimSpace(class.Filename)
				if name == "" {
					name = strings.TrimSpace(class.Name)
				}
				if name == "" {
					continue
				}
				name = path.Clean(strings.ReplaceAll(name, "\\", "/"))

				lines, ok := files[name]
				if !ok {
					lines = map[int]int{}
					files[name] = lines
					order = append(order, name)
				}
				for _, line := range class.Lines {
					// Igual que en LCOV: insertar aunque hits sea 0.
					if existing, seen := lines[line.Number]; !seen || line.Hits > existing {
						lines[line.Number] = line.Hits
					}
				}
			}
			walk(pkg.Nested)
		}
	}
	walk(document.Packages)

	report := Report{Unit: UnitLines, Files: make([]FileReport, 0, len(order))}
	for _, name := range order {
		entry := FileReport{Path: name}
		for _, hits := range files[name] {
			entry.Total++
			if hits > 0 {
				entry.Covered++
			}
		}
		report.Files = append(report.Files, entry)
	}

	return report, nil
}
