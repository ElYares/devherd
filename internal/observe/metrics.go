package observe

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// IssueCount es el numero de issues y de eventos de un proyecto en un nivel.
// Las dos etiquetas son conjuntos acotados: un proyecto por directorio y cuatro o
// cinco niveles. Etiquetar por mensaje o por fingerprint haria explotar la serie,
// que es la forma clasica de inutilizar un Prometheus.
type IssueCount struct {
	Project string
	Level   string
	Issues  int
	Events  int
}

// ContainerCount son los reinicios acumulados de un contenedor.
type ContainerCount struct {
	Project  string
	Service  string
	Name     string
	Restarts int
}

// MetricsSnapshot es todo lo que Observe puede publicar como serie, leido de una
// vez. Se toma completo y no metrica a metrica para que un scrape no vea unos
// contadores de antes y otros de despues del mismo evento.
type MetricsSnapshot struct {
	// TakenAt es cuando se leyo, para poder decir la edad del dato.
	TakenAt time.Time
	// Issues son los conteos por proyecto y nivel.
	Issues []IssueCount
	// Containers son los reinicios por contenedor.
	Containers []ContainerCount
	// CollectorUptime es cuanto lleva viva la corrida actual del collector.
	CollectorUptime time.Duration
	// CollectorRunning dice si hay una corrida en curso. Sin corridas registradas
	// no se sabe nada, que no es lo mismo que saber que esta caido.
	CollectorRunning bool
	// GapSeconds24h son los segundos sin collector escuchando en las ultimas 24 h.
	// Es la metrica que dice si un hueco en las graficas es una aplicacion sana o
	// un collector muerto.
	GapSeconds24h float64
}

// metricsGapWindow es la ventana sobre la que se reporta la falta de cobertura.
// Un dia es lo que abarca una jornada de trabajo mirando graficas.
const metricsGapWindow = 24 * time.Hour

// metricsGapFloor descarta los huecos de reinicio, igual que hace `observe status`.
// Un contador que sube unos segundos cada vez que se reinicia el collector no
// distingue una caida real de un despliegue.
const metricsGapFloor = 2 * time.Minute

// Metrics agrega los contadores en SQL, no listando y contando en Go. Los metodos
// de listado llevan LIMIT —20 por defecto en ListIssues— asi que contar con ellos
// daria un numero que parece cierto y no lo es.
func (s Store) Metrics(ctx context.Context) (MetricsSnapshot, error) {
	now := s.nowUTC()
	snapshot := MetricsSnapshot{TakenAt: now}

	issues, err := s.issueCounts(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	snapshot.Issues = issues

	containers, err := s.containerCounts(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	snapshot.Containers = containers

	session, found, err := s.LastCollectorSession(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	if found {
		// Viva es la corrida cuyo ultimo latido es reciente. Una corrida que quedo
		// abierta porque el proceso murio de golpe no cuenta como en curso, y su
		// uptime seria una mentira que crece sola.
		if now.Sub(session.LastSeen) <= metricsGapFloor {
			snapshot.CollectorRunning = true
			snapshot.CollectorUptime = now.Sub(session.StartedAt)
		}
	}

	gaps, err := s.CoverageGaps(ctx, now.Add(-metricsGapWindow), metricsGapFloor)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	for _, gap := range gaps {
		snapshot.GapSeconds24h += gap.Duration().Seconds()
	}

	return snapshot, nil
}

// issueCounts cuenta issues y sus eventos por proyecto y nivel. `event_count` ya
// vive en la fila del issue, asi que no hace falta cruzar con la tabla de eventos.
func (s Store) issueCounts(ctx context.Context) ([]IssueCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project, level, COUNT(*), COALESCE(SUM(event_count), 0)
		FROM issues
		GROUP BY project, level
		ORDER BY project ASC, level ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("count observe issues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make([]IssueCount, 0, 8)
	for rows.Next() {
		var count IssueCount
		if err := rows.Scan(&count.Project, &count.Level, &count.Issues, &count.Events); err != nil {
			return nil, fmt.Errorf("scan observe issue count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe issue counts: %w", err)
	}

	return counts, nil
}

// containerCounts son los reinicios por contenedor conocido.
func (s Store) containerCounts(ctx context.Context) ([]ContainerCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project, service, name, restart_count
		FROM containers
		ORDER BY project ASC, service ASC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("count observe containers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make([]ContainerCount, 0, 8)
	for rows.Next() {
		var count ContainerCount
		if err := rows.Scan(&count.Project, &count.Service, &count.Name, &count.Restarts); err != nil {
			return nil, fmt.Errorf("scan observe container count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observe container counts: %w", err)
	}

	// El orden ya viene de SQL, pero fijarlo aqui tambien deja el resultado estable
	// aunque manana cambie la consulta: un scrape que reordena sus lineas es
	// legal para Prometheus y molesto para cualquiera que lea el texto a ojo.
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Project != counts[j].Project {
			return counts[i].Project < counts[j].Project
		}
		if counts[i].Service != counts[j].Service {
			return counts[i].Service < counts[j].Service
		}

		return counts[i].Name < counts[j].Name
	})

	return counts, nil
}
