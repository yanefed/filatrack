package sqlite

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// DryingRepo реализует repository.DryingSessionRepository поверх SQLite.
type DryingRepo struct {
	logger *slog.Logger
	db     *DB
}

func NewDryingRepo(db *DB) repository.DryingSessionRepository {
	return &DryingRepo{db: db}
}

func NewDryingRepoWithLogger(db *DB, logger *slog.Logger,
) repository.DryingSessionRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &DryingRepo{db: db, logger: logger}
}

func (r *DryingRepo) Create(ctx context.Context, session *domain.DryingSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO drying_sessions (id, spool_id, temperature_c, duration_hours, notes, dried_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.SpoolID, session.TemperatureC, session.DurationHours,
		session.Notes, session.DriedAt, session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting drying session: %w", err)
	}
	return nil
}

func (r *DryingRepo) List(ctx context.Context, spoolID string) ([]domain.DryingSession, error) {
	rows, err := r.db.Conn().QueryContext(ctx,
		`SELECT id, spool_id, temperature_c, duration_hours, notes, dried_at, created_at
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
		var s domain.DryingSession
		if err := rows.Scan(&s.ID, &s.SpoolID, &s.TemperatureC, &s.DurationHours,
			&s.Notes, &s.DriedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning drying session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *DryingRepo) GetLastDrying(ctx context.Context, spoolID string) (*domain.DryingSession, error) {
	row := r.db.Conn().QueryRowContext(ctx,
		`SELECT id, spool_id, temperature_c, duration_hours, notes, dried_at, created_at
		 FROM drying_sessions WHERE spool_id = ? ORDER BY dried_at DESC LIMIT 1`, spoolID)

	var s domain.DryingSession
	err := row.Scan(&s.ID, &s.SpoolID, &s.TemperatureC, &s.DurationHours,
		&s.Notes, &s.DriedAt, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting last drying: %w", err)
	}
	return &s, nil
}
