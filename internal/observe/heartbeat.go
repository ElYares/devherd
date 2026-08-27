package observe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CollectorSession es una corrida del collector: cuando arranco y cuando se le
// vio por ultima vez.
type CollectorSession struct {
	ID        int64     `json:"id"`
	StartedAt time.Time `json:"started_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// Duration es cuanto duro la corrida.
func (s CollectorSession) Duration() time.Duration {
	return s.LastSeen.Sub(s.StartedAt)
}

// CoverageGap es un intervalo en el que el collector no estuvo escuchando. Es el
// dato que convierte un `observe issues` vacio de ambiguo en concluyente: dentro
// del hueco no hay issues porque nadie los recibio, no porque no ocurrieran.
type CoverageGap struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Duration es cuanto tiempo estuvo el collector sin escuchar.
func (g CoverageGap) Duration() time.Duration {
	return g.To.Sub(g.From)
}

// StartCollectorSession abre una corrida nueva y devuelve su id. Se llama una vez
// al arrancar el collector; los latidos posteriores la actualizan.
func (s Store) StartCollectorSession(ctx context.Context) (int64, error) {
	now := s.nowUTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO collector_sessions (started_at, last_seen) VALUES (?, ?)
	`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("start collector session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("start collector session: %w", err)
	}

	return id, nil
}

// Heartbeat marca que el collector sigue vivo. La resolucion del registro es el
// intervalo con que se llama: si el proceso muere de golpe, el ultimo latido
// escrito es lo mas cerca que se puede estar del momento real.
func (s Store) Heartbeat(ctx context.Context, sessionID int64) error {
	if sessionID <= 0 {
		return fmt.Errorf("heartbeat: invalid session id %d", sessionID)
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE collector_sessions SET last_seen = ? WHERE id = ?
	`, s.nowUTC().Format(time.RFC3339Nano), sessionID)
	if err != nil {
		return fmt.Errorf("collector heartbeat: %w", err)
	}

	return nil
}

// CollectorSessions devuelve las corridas que tocan la ventana pedida, de mas
// antigua a mas reciente. El orden ascendente es el que necesita CoverageGaps:
// un hueco solo tiene sentido entre una corrida y la siguiente.
func (s Store) CollectorSessions(ctx context.Context, since time.Time) ([]CollectorSession, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, started_at, last_seen
		FROM collector_sessions
		WHERE datetime(last_seen) >= datetime(?)
		ORDER BY datetime(started_at) ASC, id ASC
	`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("list collector sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sessions := make([]CollectorSession, 0, 8)
	for rows.Next() {
		var (
			session           CollectorSession
			started, lastSeen string
		)
		if err := rows.Scan(&session.ID, &started, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan collector session: %w", err)
		}
		session.StartedAt, _ = parseAlertDeliveryTime(started)
		session.LastSeen, _ = parseAlertDeliveryTime(lastSeen)
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read collector sessions: %w", err)
	}

	return sessions, nil
}

// LastCollectorSession es la corrida mas reciente, util para saber desde cuando
// esta arriba el collector de ahora.
func (s Store) LastCollectorSession(ctx context.Context) (CollectorSession, bool, error) {
	var (
		session           CollectorSession
		started, lastSeen string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, started_at, last_seen
		FROM collector_sessions
		ORDER BY datetime(started_at) DESC, id DESC
		LIMIT 1
	`).Scan(&session.ID, &started, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return CollectorSession{}, false, nil
	}
	if err != nil {
		return CollectorSession{}, false, fmt.Errorf("last collector session: %w", err)
	}

	session.StartedAt, _ = parseAlertDeliveryTime(started)
	session.LastSeen, _ = parseAlertDeliveryTime(lastSeen)

	return session, true, nil
}

// CoverageGaps son los intervalos sin collector escuchando dentro de la ventana,
// mas largos que minGap.
//
// minGap existe porque reiniciar el collector deja un hueco de segundos que no le
// interesa a nadie. Avisar de esos convierte el aviso en ruido, y un aviso que se
// ignora no avisa de nada.
func (s Store) CoverageGaps(ctx context.Context, since time.Time, minGap time.Duration) ([]CoverageGap, error) {
	sessions, err := s.CollectorSessions(ctx, since)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	gaps := make([]CoverageGap, 0, 4)

	// El hueco inicial: desde el principio de la ventana hasta que arranco la
	// primera corrida que la toca. Es el mas facil de olvidar y el mas grande
	// cuando alguien enciende el collector despues de dias sin usarlo.
	if first := sessions[0].StartedAt; first.After(since) {
		if gap := (CoverageGap{From: since, To: first}); gap.Duration() > minGap {
			gaps = append(gaps, gap)
		}
	}

	for i := 1; i < len(sessions); i++ {
		previous := sessions[i-1].LastSeen
		next := sessions[i].StartedAt
		// Dos corridas solapadas —dos collectores a la vez— no dejan hueco.
		if !next.After(previous) {
			continue
		}
		if gap := (CoverageGap{From: previous, To: next}); gap.Duration() > minGap {
			gaps = append(gaps, gap)
		}
	}

	return gaps, nil
}
