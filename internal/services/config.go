package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	servicestemplates "github.com/devherd/devherd/templates/services"
)

// ManagedFile es un archivo de configuracion que DevHerd escribe dentro del stack
// compartido para que un servicio funcione.
//
// La linea entre esto y el compose es deliberada: **el compose es de DevHerd, la
// configuracion es del usuario.** El compose es el catalogo de lo que DevHerd
// ofrece y se regenera siempre, porque si quedara congelado con la edicion de
// alguien, una version nueva del binario no podria ofrecer un servicio nuevo. Los
// archivos de configuracion son el ajuste de cada quien y se respetan.
type ManagedFile struct {
	// Path es la ruta relativa al directorio del stack compartido.
	Path string
	// Content es la plantilla que DevHerd escribe la primera vez.
	Content string
	// GeneratedOnce marca un archivo cuyo contenido se genera y **no se puede
	// comparar** contra una plantilla: un token aleatorio es distinto cada vez que
	// se construye. Sin esto, cada arranque lo veria como "editado por el usuario"
	// y soltaria un aviso que no significa nada, que es como se ensena a ignorar
	// los avisos. Si existe, se deja como esta y en silencio.
	GeneratedOnce bool
}

// ServiceOptions son los datos del entorno que necesitan las plantillas.
//
// HU-011 dejo las plantillas parametrizables fuera de alcance a proposito, y
// Prometheus es el caso que obliga a tenerlas: su unico contenido util es una
// direccion que solo se conoce en tiempo de ejecucion.
type ServiceOptions struct {
	// CollectorAddr es la direccion del collector de Observe **alcanzable desde un
	// contenedor**. No es 127.0.0.1: desde dentro de un contenedor, loopback es el
	// propio contenedor, y apuntar ahi deja un target caido sin explicacion.
	CollectorAddr string
	// Workspace es el directorio del host que ve Jupyter. Uno solo y arriba del
	// todo: la gracia de un entorno global es abrir el notebook de cualquier
	// proyecto sin reconfigurar nada.
	Workspace string
	// UID y GID son los del usuario del host. Sin ellos, lo que el notebook
	// escriba en el workspace acabaria perteneciendo a otro dueno.
	UID int
	GID int
	// SlackConfigured dice si el usuario puso un webhook de Slack en el .env del
	// stack. Es un booleano y no el webhook porque el valor nunca pasa por aqui:
	// lo lee Grafana de su entorno. Lo unico que decide este campo es **si el
	// contact point se escribe**, y esa decision no necesita el secreto.
	SlackConfigured bool
}

// serviceFiles declara la configuracion de cada servicio. Redis y Mailpit no
// necesitan ninguna, y por eso el problema de las escrituras no se veia hasta que
// llego Prometheus.
var serviceFiles = map[string]func(ServiceOptions) ([]ManagedFile, error){
	"prometheus": prometheusFiles,
	"grafana":    grafanaFiles,
	"jupyter":    jupyterFiles,
}

// jupyterFiles escribe el .env que docker compose lee para saber que directorio
// montar y con que token proteger el servidor.
//
// Va en un .env y no en el compose porque el compose es de DevHerd y se regenera
// siempre: el directorio de trabajo es de cada quien y su token no se puede volver
// a generar sin invalidar la sesion abierta.
func jupyterFiles(opts ServiceOptions) ([]ManagedFile, error) {
	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		return nil, fmt.Errorf("a workspace directory is required to configure Jupyter")
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	content := "# Escrito por DevHerd la primera vez que arrancaste Jupyter.\n" +
		"#\n" +
		"# DEVHERD_WORKSPACE es el directorio del host que Jupyter monta en /home/jovyan/work.\n" +
		"# Cambialo aqui si quieres ver otro arbol; DevHerd respeta este archivo y no lo pisa.\n" +
		"#\n" +
		"# JUPYTER_TOKEN protege el servidor. **No lo quites.** Un Jupyter sin token es\n" +
		"# ejecucion de codigo arbitrario con escritura sobre todo lo que hay montado, y\n" +
		"# aqui esta montado tu codigo entero.\n" +
		"#\n" +
		"# El uid y el gid del host: lo que escribas desde el notebook te sigue\n" +
		"# perteneciendo a ti fuera del contenedor.\n" +
		"DEVHERD_WORKSPACE=" + workspace + "\n" +
		"DEVHERD_UID=" + strconv.Itoa(opts.UID) + "\n" +
		"DEVHERD_GID=" + strconv.Itoa(opts.GID) + "\n" +
		"JUPYTER_TOKEN=" + token + "\n"

	return []ManagedFile{{Path: ".env", Content: content, GeneratedOnce: true}}, nil
}

// generateToken produce el token del servidor de Jupyter.
func generateToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate the Jupyter token: %w", err)
	}

	return hex.EncodeToString(raw), nil
}

// DefaultWorkspace es el directorio que Jupyter monta si nadie dice otra cosa.
// `~/develop` cuando existe, porque es donde suele vivir el arbol de proyectos, y
// el home si no: montar la raiz del sistema seria peor que preguntar.
func DefaultWorkspace(home string) string {
	if home == "" {
		return ""
	}

	candidate := filepath.Join(home, "develop")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}

	return home
}

// grafanaFiles son los tres archivos de provisioning: la fuente de datos, el
// proveedor de tableros y el tablero.
//
// **El tablero es lo que decide si empaquetar Grafana valio la pena.** Con
// datasource y sin tableros, el usuario se queda exactamente donde estaba: habria
// cambiado "configura Prometheus a mano" por "construye un tablero a mano".
//
// Ninguno lleva plantilla: Grafana alcanza a Prometheus por su alias de red, que
// no cambia. Una IP si cambiaria al recrear la red, que es la misma trampa que ya
// costo un falso positivo en Observe.
func grafanaFiles(opts ServiceOptions) ([]ManagedFile, error) {
	files := []ManagedFile{
		{Path: "grafana/datasources/prometheus.yml", Content: servicestemplates.GrafanaDatasource},
		{Path: "grafana/dashboards/devherd.yml", Content: servicestemplates.GrafanaDashboards},
		{Path: "grafana/dashboards/devherd/devherd-observe.json", Content: servicestemplates.GrafanaDashboard},
		{Path: "grafana/alerting/devherd-rules.yml", Content: servicestemplates.GrafanaAlertingRules},
	}

	// El contact point de Slack solo se escribe si hay webhook. **Escribirlo sin
	// el impide que Grafana arranque**, y no de una forma que se entienda: un
	// $__env{} sin definir resuelve a cadena vacia, la validacion del receptor
	// pide un `url` que ya no esta y el proceso sale con codigo 1. Un servicio
	// compartido no puede caerse porque alguien no use Slack.
	if opts.SlackConfigured {
		files = append(files, ManagedFile{
			Path:    "grafana/alerting/devherd-slack.yml",
			Content: servicestemplates.GrafanaAlertingSlack,
		})
	}

	return files, nil
}

// prometheusFiles arma el prometheus.yml con el collector ya apuntado. Escribirlo
// a mano exige saber que la direccion no es loopback, cual es el gateway de la red
// compartida y donde va el archivo: justo el trabajo que DevHerd existe para
// ahorrar, y el unico motivo por el que empaquetar Prometheus tiene sentido.
func prometheusFiles(opts ServiceOptions) ([]ManagedFile, error) {
	addr := strings.TrimSpace(opts.CollectorAddr)
	if addr == "" {
		return nil, fmt.Errorf("the Observe collector address is required to configure Prometheus")
	}

	tmpl, err := template.New("prometheus.yml").Parse(servicestemplates.PrometheusConfig)
	if err != nil {
		return nil, fmt.Errorf("parse the Prometheus configuration template: %w", err)
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, struct{ CollectorAddr string }{CollectorAddr: addr}); err != nil {
		return nil, fmt.Errorf("render the Prometheus configuration: %w", err)
	}

	return []ManagedFile{{Path: "prometheus/prometheus.yml", Content: rendered.String()}}, nil
}

// FileState dice que se hizo con un archivo administrado, para poder contarlo.
type FileState int

const (
	// FileWritten es un archivo que no existia y se creo.
	FileWritten FileState = iota
	// FileUnchanged es un archivo que ya estaba y coincide con la plantilla.
	FileUnchanged
	// FileKept es un archivo que el usuario edito y se dejo como estaba.
	FileKept
	// FileRestored es un archivo editado que se devolvio a la plantilla con force.
	FileRestored
)

// FileResult es lo que paso con un archivo administrado.
type FileResult struct {
	Path  string
	State FileState
	// Backup es la ruta de la copia, cuando se restauro una plantilla sobre una
	// edicion del usuario.
	Backup string
}

// backupSuffix es lo que se le agrega al archivo antes de pisarlo con --force.
// Restaurar sin copia convierte una bandera en una perdida de trabajo.
const backupSuffix = ".bak"

// ServiceFiles devuelve la configuracion declarada por un servicio, o nada.
func ServiceFiles(service string, opts ServiceOptions) ([]ManagedFile, error) {
	build, ok := serviceFiles[service]
	if !ok {
		return nil, nil
	}

	files, err := build(opts)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return files, nil
}

// NeedsCollector dice si un servicio necesita saber donde escucha el collector.
// Se consulta **antes** de arrancar nada, para poder avisar cuando la direccion no
// sirve en vez de dejar un target caido que se descubre media hora despues.
func NeedsCollector(service string) bool {
	return service == "prometheus"
}

// ensureServiceFiles escribe la configuracion de un servicio respetando lo que el
// usuario haya editado. Devuelve que paso con cada archivo para que el comando lo
// pueda decir: escribir configuracion en silencio dentro del directorio de alguien
// es como se pierde una tarde buscando por que un servicio no toma sus ajustes.
func (m Manager) ensureServiceFiles(service string, opts StartOptions) ([]FileResult, error) {
	files, err := ServiceFiles(service, ServiceOptions{
		CollectorAddr:   opts.CollectorAddr,
		Workspace:       opts.Workspace,
		UID:             opts.UID,
		GID:             opts.GID,
		SlackConfigured: SlackConfigured(m.dir),
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	results := make([]FileResult, 0, len(files))
	for _, file := range files {
		result, err := m.ensureFile(file, opts.Force)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (m Manager) ensureFile(file ManagedFile, force bool) (FileResult, error) {
	path := filepath.Join(m.dir, filepath.FromSlash(file.Path))
	result := FileResult{Path: path}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileResult{}, fmt.Errorf("create directory for %s: %w", file.Path, err)
	}

	existing, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return FileResult{}, fmt.Errorf("write %s: %w", file.Path, err)
		}
		result.State = FileWritten

		return result, nil

	case err != nil:
		return FileResult{}, fmt.Errorf("read %s: %w", file.Path, err)
	}

	// Un archivo generado no se compara: su contenido es distinto en cada
	// construccion por definicion. Existir ya es la respuesta.
	if file.GeneratedOnce || string(existing) == file.Content {
		result.State = FileUnchanged

		return result, nil
	}

	if !force {
		// La edicion del usuario manda. Volver a escribir la plantilla aqui es
		// exactamente el bug que esta historia viene a arreglar.
		result.State = FileKept

		return result, nil
	}

	backup := path + backupSuffix
	if err := os.WriteFile(backup, existing, 0o644); err != nil {
		return FileResult{}, fmt.Errorf("back up %s: %w", file.Path, err)
	}
	if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
		return FileResult{}, fmt.Errorf("restore %s: %w", file.Path, err)
	}
	result.State = FileRestored
	result.Backup = backup

	return result, nil
}

// DescribeFileResults arma el aviso para quien corre el comando. Devuelve vacio
// cuando no hay nada que decir: un aviso que sale siempre se aprende a ignorar.
func DescribeFileResults(results []FileResult) string {
	var lines []string
	for _, result := range results {
		switch result.State {
		case FileKept:
			lines = append(lines,
				fmt.Sprintf("keeping your edited %s; it differs from the DevHerd template "+
					"(--force restores it, keeping a %s copy)", result.Path, backupSuffix))
		case FileRestored:
			lines = append(lines,
				fmt.Sprintf("restored %s from the DevHerd template; your version is at %s",
					result.Path, result.Backup))
		case FileWritten:
			lines = append(lines, fmt.Sprintf("wrote %s", result.Path))
		case FileUnchanged:
		}
	}

	return strings.Join(lines, "\n")
}

// slackWebhookVar es la variable del .env donde vive el webhook de Slack. Va en
// el .env y no en el compose por lo de siempre: el compose es de DevHerd y se
// regenera, el secreto es del usuario y se queda.
const slackWebhookVar = "DEVHERD_SLACK_WEBHOOK"

// SlackConfigured dice si el .env del stack define un webhook de Slack con algun
// valor. No valida que la URL sirva —eso lo dira Grafana al intentar entregar—,
// solo distingue "configurado" de "ausente", que es la unica pregunta que decide
// si el contact point se escribe.
//
// Un .env que no existe todavia no es un error: significa que nadie ha arrancado
// Jupyter ni ha puesto un webhook, y la respuesta correcta es que no hay Slack.
func SlackConfigured(dir string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		// Una linea comentada es justo como queda un webhook que alguien desactivo
		// sin borrarlo. Tomarla por buena escribiria el contact point y dejaria a
		// Grafana sin arrancar.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(name) != slackWebhookVar {
			continue
		}
		if strings.TrimSpace(value) != "" {
			return true
		}
	}

	return false
}
