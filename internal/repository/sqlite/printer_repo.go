package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"

	"github.com/google/uuid"
)

type PrinterRepo struct {
	db *DB
}

func NewPrinterRepo(db *DB) repository.PrinterRepository {
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
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO printers (id, name, kind, moonraker_url, api_key, enabled, active_spool_id, state, current_file, progress, last_seen_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		printer.ID, printer.Name, string(printer.Kind), printer.MoonrakerURL, printer.APIKey,
		boolToInt(printer.Enabled), nullString(printer.ActiveSpoolID), string(printer.State),
		printer.CurrentFile, printer.Progress, nullTime(printer.LastSeenAt),
		printer.CreatedAt, printer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting printer: %w", err)
	}
	return nil
}

func (r *PrinterRepo) GetByID(ctx context.Context, id string) (*domain.Printer, error) {
	row := r.db.Conn().QueryRowContext(ctx,
		`SELECT id, name, kind, moonraker_url, api_key, enabled, COALESCE(active_spool_id, ''), state, current_file, progress, last_seen_at, created_at, updated_at
		 FROM printers WHERE id = ?`, id)
	printer, err := scanPrinter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("printer not found: %s", id)
		}
		return nil, fmt.Errorf("querying printer: %w", err)
	}
	return printer, nil
}

func (r *PrinterRepo) List(ctx context.Context) ([]domain.Printer, error) {
	rows, err := r.db.Conn().QueryContext(ctx,
		`SELECT id, name, kind, moonraker_url, api_key, enabled, COALESCE(active_spool_id, ''), state, current_file, progress, last_seen_at, created_at, updated_at
		 FROM printers ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing printers: %w", err)
	}
	defer rows.Close()

	var printers []domain.Printer
	for rows.Next() {
		printer, err := scanPrinterRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning printer: %w", err)
		}
		printers = append(printers, *printer)
	}
	return printers, rows.Err()
}

func (r *PrinterRepo) Update(ctx context.Context, printer *domain.Printer) error {
	printer.UpdatedAt = time.Now()
	result, err := r.db.Conn().ExecContext(ctx,
		`UPDATE printers
		 SET name=?, kind=?, moonraker_url=?, api_key=?, enabled=?, active_spool_id=?, state=?, current_file=?, progress=?, last_seen_at=?, updated_at=?
		 WHERE id=?`,
		printer.Name, string(printer.Kind), printer.MoonrakerURL, printer.APIKey,
		boolToInt(printer.Enabled), nullString(printer.ActiveSpoolID), string(printer.State),
		printer.CurrentFile, printer.Progress, nullTime(printer.LastSeenAt), printer.UpdatedAt,
		printer.ID,
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
	result, err := r.db.Conn().ExecContext(ctx,
		`UPDATE printers SET active_spool_id = ?, updated_at = ? WHERE id = ?`,
		nullString(spoolID), time.Now(), printerID,
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
	result, err := r.db.Conn().ExecContext(ctx,
		`UPDATE printers SET state=?, current_file=?, progress=?, last_seen_at=?, updated_at=? WHERE id=?`,
		string(printer.State), printer.CurrentFile, printer.Progress, nullTime(printer.LastSeenAt),
		printer.UpdatedAt, printer.ID,
	)
	if err != nil {
		return fmt.Errorf("updating printer status: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("printer not found: %s", printer.ID)
	}
	return nil
}

func scanPrinter(row *sql.Row) (*domain.Printer, error) {
	var p domain.Printer
	var kind, state string
	var enabled int
	var lastSeen sql.NullTime
	err := row.Scan(&p.ID, &p.Name, &kind, &p.MoonrakerURL, &p.APIKey, &enabled,
		&p.ActiveSpoolID, &state, &p.CurrentFile, &p.Progress, &lastSeen,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Kind = domain.PrinterKind(kind)
	p.State = domain.PrinterState(state)
	p.Enabled = enabled != 0
	if lastSeen.Valid {
		p.LastSeenAt = lastSeen.Time
	}
	return &p, nil
}

func scanPrinterRows(rows *sql.Rows) (*domain.Printer, error) {
	var p domain.Printer
	var kind, state string
	var enabled int
	var lastSeen sql.NullTime
	err := rows.Scan(&p.ID, &p.Name, &kind, &p.MoonrakerURL, &p.APIKey, &enabled,
		&p.ActiveSpoolID, &state, &p.CurrentFile, &p.Progress, &lastSeen,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Kind = domain.PrinterKind(kind)
	p.State = domain.PrinterState(state)
	p.Enabled = enabled != 0
	if lastSeen.Valid {
		p.LastSeenAt = lastSeen.Time
	}
	return &p, nil
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
