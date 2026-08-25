package coverage

import (
	"path"
	"strings"
)

// jacocoParser lee el XML de JaCoCo, que es lo que produce Maven o Gradle en un
// proyecto Java.
//
//	<report><package name="com/foo"><sourcefile name="Foo.java">
//	  <counter type="LINE" missed="5" covered="20"/>
//
// Su raiz <report> es unica entre los cinco formatos, asi que detectarlo es
// directo. Se usa el contador LINE y no INSTRUCTION: las instrucciones de bytecode
// no son comparables con lo que miden los demas formatos.
type jacocoParser struct{}

type jacocoReport struct {
	Packages []jacocoPackage `xml:"package"`
}

type jacocoPackage struct {
	Name        string             `xml:"name,attr"`
	SourceFiles []jacocoSourceFile `xml:"sourcefile"`
}

type jacocoSourceFile struct {
	Name     string          `xml:"name,attr"`
	Counters []jacocoCounter `xml:"counter"`
	Lines    []jacocoLine    `xml:"line"`
}

type jacocoCounter struct {
	Type    string `xml:"type,attr"`
	Missed  int    `xml:"missed,attr"`
	Covered int    `xml:"covered,attr"`
}

type jacocoLine struct {
	Number        int `xml:"nr,attr"`
	MissedInstr   int `xml:"mi,attr"`
	CoveredInstr  int `xml:"ci,attr"`
	MissedBranch  int `xml:"mb,attr"`
	CoveredBranch int `xml:"cb,attr"`
}

func (jacocoParser) Name() string { return "jacoco" }

func (jacocoParser) Detect(data []byte) bool {
	root, _, ok := xmlShape(data)

	return ok && root == "report"
}

func (jacocoParser) Parse(data []byte) (Report, error) {
	var document jacocoReport
	if err := decodeXML(data, &document); err != nil {
		return Report{}, err
	}

	report := Report{Unit: UnitLines}
	for _, pkg := range document.Packages {
		prefix := strings.Trim(strings.ReplaceAll(pkg.Name, "\\", "/"), "/")
		for _, source := range pkg.SourceFiles {
			name := source.Name
			if prefix != "" {
				name = path.Join(prefix, name)
			}

			entry := FileReport{Path: name}

			// El contador LINE del sourcefile es el resumen que publica JaCoCo.
			// Si falta, se reconstruye desde las lineas: una linea cuenta como
			// cubierta cuando ejecuto al menos una instruccion.
			counted := false
			for _, counter := range source.Counters {
				if counter.Type != "LINE" {
					continue
				}
				entry.Total = counter.Missed + counter.Covered
				entry.Covered = counter.Covered
				counted = true

				break
			}

			if !counted {
				for _, line := range source.Lines {
					entry.Total++
					if line.CoveredInstr > 0 {
						entry.Covered++
					}
				}
			}

			report.Files = append(report.Files, entry)
		}
	}

	return report, nil
}
