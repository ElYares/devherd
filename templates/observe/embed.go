// Package observetemplates agrupa los reporters que `devherd observe attach
// --reporter` escribe dentro de un proyecto. Son archivos reales, no plantillas
// con marcadores, para que se puedan leer y editar igual que el codigo del
// proyecto que los recibe.
package observetemplates

import "embed"

const ReporterLaravel = "reporter-laravel.php"

// Files contiene los reporters embebidos en el binario de DevHerd.
//
//go:embed reporter-laravel.php
var Files embed.FS
