package version

import "fmt"

// Estas variables se inyectan en build con -ldflags (ver Makefile).
var (
	Version = "0.1.0-alpha"
	Commit  = "dev"
	Date    = "unknown"
)

// String devuelve únicamente la versión semántica.
func String() string {
	return Version
}

// Long devuelve la versión enriquecida con commit y fecha de build,
// usada como salida de `devherd --version`.
func Long() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
