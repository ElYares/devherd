package coverage

import (
	"fmt"
	"path"
	"strings"
)

// cloverParser lee Clover, que es lo que emite PHPUnit con --coverage-clover. Es
// el formato de Laravel.
//
//	<coverage><project><file name="/app/Foo.php">
//	  <metrics statements="10" coveredstatements="7"/>
//	  <line num="12" type="stmt" count="1"/>
//
// Comparte el elemento raiz <coverage> con Cobertura; lo que los separa es el
// primer hijo, <project> aqui y <packages> alla.
type cloverParser struct{}

type cloverDocument struct {
	Project cloverProject `xml:"project"`
}

type cloverProject struct {
	Files    []cloverFile    `xml:"file"`
	Packages []cloverPackage `xml:"package"`
}

type cloverPackage struct {
	Name  string       `xml:"name,attr"`
	Files []cloverFile `xml:"file"`
}

type cloverFile struct {
	Name    string        `xml:"name,attr"`
	Path    string        `xml:"path,attr"`
	Metrics cloverMetrics `xml:"metrics"`
	Lines   []cloverLine  `xml:"line"`
}

type cloverMetrics struct {
	Statements        int  `xml:"statements,attr"`
	CoveredStatements int  `xml:"coveredstatements,attr"`
	Present           bool `xml:"-"`
}

type cloverLine struct {
	Num   int    `xml:"num,attr"`
	Type  string `xml:"type,attr"`
	Count int    `xml:"count,attr"`
}

func (cloverParser) Name() string { return "clover" }

func (cloverParser) Detect(data []byte) bool {
	root, child, ok := xmlShape(data)

	return ok && root == "coverage" && child == "project"
}

func (cloverParser) Parse(data []byte) (Report, error) {
	var document cloverDocument
	if err := decodeXML(data, &document); err != nil {
		return Report{}, err
	}

	files := document.Project.Files
	for _, pkg := range document.Project.Packages {
		files = append(files, pkg.Files...)
	}

	report := Report{Unit: UnitStatements, Files: make([]FileReport, 0, len(files))}
	for _, file := range files {
		name := strings.TrimSpace(file.Name)
		if name == "" {
			name = strings.TrimSpace(file.Path)
		}
		if name == "" {
			return Report{}, fmt.Errorf("clover file entry without a name")
		}

		entry := FileReport{Path: path.Clean(strings.ReplaceAll(name, "\\", "/"))}

		// Se cuentan las lineas de tipo stmt cuando estan, porque es el dato
		// verificable. <metrics> es un resumen y se usa solo si no hay detalle.
		counted := false
		for _, line := range file.Lines {
			if line.Type != "" && line.Type != "stmt" {
				continue
			}
			counted = true
			entry.Total++
			if line.Count > 0 {
				entry.Covered++
			}
		}

		if !counted {
			entry.Total = file.Metrics.Statements
			entry.Covered = file.Metrics.CoveredStatements
		}

		report.Files = append(report.Files, entry)
	}

	return report, nil
}
