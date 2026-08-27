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

// GrafanaDatasource declara el Prometheus compartido como fuente de datos. No
// lleva plantilla: se apunta por alias de red, que no cambia.
//
//go:embed grafana-datasource.yml
var GrafanaDatasource string

// GrafanaDashboards declara de donde salen los tableros provisionados.
//
//go:embed grafana-dashboards.yml
var GrafanaDashboards string

// GrafanaDashboard es el tablero de DevHerd Observe. Es lo que decide si
// empaquetar Grafana valio la pena: con datasource y sin tableros, el usuario se
// queda exactamente donde estaba.
//
//go:embed grafana-dashboard.json
var GrafanaDashboard string
