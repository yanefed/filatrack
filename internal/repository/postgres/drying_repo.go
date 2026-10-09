package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"log/slog"

	"github.com/yanefed/filatrack/internal/repository/entity"

	"github.com/google/uuid"
)

// DryingRepo реализует port.DryingSessionRepository поверх SQLite.
type DryingRepo struct {
	logger *slog.Logger
	db     *DB
}

func NewDryingRepo(db *DB) port.DryingSessionRepository {
	return NewDryingRepoWithLogger(db, nil)
}

func NewDryingRepoWithLogger(db *DB, logger *slog.Logger,
) port.DryingSessionRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &DryingRepo{db: db, logger: logger}
}

func (r *DryingRepo) Create(ctx context.Context, session *domain.DryingSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	persisted := entity.DryingSessionFromDomain(*session)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO drying_sessions (id, spool_id, temperature_c, duration_hours, dried_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		persisted.ID, persisted.SpoolID, persisted.TemperatureC, persisted.DurationHours, persisted.DriedAt, persisted.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting drying session: %w", err)
	}
	return nil
}

func (r *DryingRepo) List(ctx context.Context, spoolID string) ([]domain.DryingSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, spool_id, temperature_c, duration_hours, dried_at, created_at
		 FROM drying_sessions WHERE spool_id = ? ORDER BY dried_at DESC`, spoolID)
	if err != nil {
		return nil, fmt.Errorf("listing drying sessions: %w", err)
	}
	defer func(rows *sql.Rows) {
		if err := rows.Close(); err != nil {
			r.logger.Error("failed to close rows", "error", err)
		}

	}(rows)

	var sessions []domain.DryingSession
	for rows.Next() {
		persisted, err := entity.ScanDryingSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning drying session: %w", err)
		}
		sessions = append(sessions, persisted.Domain())
	}
	return sessions, rows.Err()
}

func (r *DryingRepo) GetLastDrying(ctx context.Context, spoolID string) (*domain.DryingSession, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, spool_id, temperature_c, duration_hours, dried_at, created_at
		 FROM drying_sessions WHERE spool_id = ? ORDER BY dried_at DESC LIMIT 1`, spoolID)

	persisted, err := entity.ScanDryingSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting last drying: %w", err)
	}
	session := persisted.Domain()
	return &session, nil
}
