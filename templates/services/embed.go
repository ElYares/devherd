// Package servicestemplates embebe las plantillas de configuracion de los
// servicios compartidos. Viven en archivos reales para que editores y linters de
// YAML las validen, igual que el compose y las migraciones.
package servicestemplates

import _ "embed"

// PrometheusConfig es el prometheus.yml que DevHerd escribe la primera vez. Lleva
// una plantilla de texto: su unico contenido util es la direccion del collector,
// que solo se conoce en tiempo de ejecucion.
//
//go:embed prometheus.yml
var PrometheusConfig string
