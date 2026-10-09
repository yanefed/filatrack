package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/repository/dbutil"
	"time"

	"github.com/yanefed/filatrack/internal/repository/entity"

	"github.com/google/uuid"
)

type PrinterRepo struct {
	db *DB
}

func NewPrinterRepo(db *DB) port.PrinterRepository {
	return &PrinterRepo{db: db}
}

func (r *PrinterRepo) Create(ctx context.Context, printer *domain.Printer) error {
	if printer.ID == "" {
		printer.ID = uuid.NewString()
	}
	if printer.Kind == "" {
		printer.Kind = domain.PrinterKindMoonraker
	}
	if printer.State == "" {
		printer.State = domain.PrinterStateUnknown
	}
	persisted := entity.PrinterFromDomain(*printer)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO printers (id, name, kind, moonraker_url, api_key, enabled, active_spool_id, state, current_file, progress, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		persisted.ID, persisted.Name, persisted.Kind, persisted.MoonrakerURL, persisted.APIKey,
		persisted.Enabled, persisted.ActiveSpoolID, persisted.State,
		persisted.CurrentFile, persisted.Progress, persisted.LastSeenAt,
		persisted.CreatedAt, persisted.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting printer: %w", err)
	}
	return nil
}

func (r *PrinterRepo) GetByID(ctx context.Context, id string) (*domain.Printer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, kind, moonraker_url, api_key, enabled, active_spool_id, state, current_file, progress, last_seen_at, created_at, updated_at
		 FROM printers WHERE id = ?`, id)
	persisted, err := entity.ScanPrinter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("printer not found: %s", id)
		}
		return nil, fmt.Errorf("querying printer: %w", err)
	}
	printer := persisted.Domain()
	return &printer, nil
}

func (r *PrinterRepo) List(ctx context.Context) (result []domain.Printer, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, kind, moonraker_url, api_key, enabled, active_spool_id, state, current_file, progress, last_seen_at, created_at, updated_at
		 FROM printers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing printers: %w", err)
	}
	defer dbutil.Close(rows, &err)

	var printers []domain.Printer
	for rows.Next() {
		persisted, err := entity.ScanPrinter(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning printer: %w", err)
		}
		printers = append(printers, persisted.Domain())
	}
	return printers, rows.Err()
}

func (r *PrinterRepo) Update(ctx context.Context, printer *domain.Printer) error {
	printer.UpdatedAt = time.Now()
	persisted := entity.PrinterFromDomain(*printer)
	result, err := r.db.ExecContext(ctx,
		`UPDATE printers
		 SET name=?, kind=?, moonraker_url=?, api_key=?, enabled=?, active_spool_id=?, state=?, current_file=?, progress=?, last_seen_at=?, updated_at=?
		 WHERE id=?`,
		persisted.Name, persisted.Kind, persisted.MoonrakerURL, persisted.APIKey,
		persisted.Enabled, persisted.ActiveSpoolID, persisted.State,
		persisted.CurrentFile, persisted.Progress, persisted.LastSeenAt, persisted.UpdatedAt,
		persisted.ID,
	)
	if err != nil {
		return fmt.Errorf("updating printer: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("printer not found: %s", printer.ID)
	}
	return nil
}

func (r *PrinterRepo) AssignSpool(ctx context.Context, printerID, spoolID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE printers SET active_spool_id = ?, updated_at = ? WHERE id = ?`,
		entity.NullString(spoolID), time.Now(), printerID,
	)
	if err != nil {
		return fmt.Errorf("assigning printer spool: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("printer not found: %s", printerID)
	}
	return nil
}

func (r *PrinterRepo) UpdateStatus(ctx context.Context, printer *domain.Printer) error {
	printer.UpdatedAt = time.Now()
	persisted := entity.PrinterFromDomain(*printer)
	result, err := r.db.ExecContext(ctx,
		`UPDATE printers SET state=?, current_file=?, progress=?, last_seen_at=?, updated_at=? WHERE id=?`,
		persisted.State, persisted.CurrentFile, persisted.Progress, persisted.LastSeenAt,
		persisted.UpdatedAt, persisted.ID,
	)
	if err != nil {
		return fmt.Errorf("updating printer status: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("printer not found: %s", printer.ID)
	}
	return nil
}
