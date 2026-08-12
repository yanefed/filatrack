package sqlite

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PrinterModelRepo struct {
	db *DB
}

func NewPrinterModelRepo(db *DB) repository.PrinterModelRepository {
	return &PrinterModelRepo{db: db}
}

func (r *PrinterModelRepo) SaveForPrinter(ctx context.Context, printerID string, models []domain.PrinterModelFile) (err error) {
	tx, err := r.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting printer models transaction: %w", err)
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	if _, err := tx.ExecContext(ctx, `DELETE FROM printer_models WHERE printer_id = ?`, printerID); err != nil {
		return fmt.Errorf("clearing printer models: %w", err)
	}

	now := time.Now()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO printer_models (
			printer_id, path, filename, modified_at, size_bytes, estimated_weight_g,
			print_duration_sec, slicer, preview_image_path, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing printer model insert: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, model := range models {
		model.PrinterID = printerID
		if model.CachedAt.IsZero() {
			model.CachedAt = now
		}
		if _, err := stmt.ExecContext(ctx,
			model.PrinterID, model.Path, model.Filename, nullTime(model.ModifiedAt), model.SizeBytes,
			model.EstimatedWeightG, int64(model.PrintDuration.Seconds()), model.Slicer,
			model.PreviewImagePath, model.CachedAt,
		); err != nil {
			return fmt.Errorf("inserting printer model %s: %w", model.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing printer models: %w", err)
	}
	return nil
}

func (r *PrinterModelRepo) List(ctx context.Context, printerID *string) ([]domain.PrinterModelFile, error) {
	query := `SELECT printer_id, path, filename, modified_at, size_bytes, estimated_weight_g, print_duration_sec, slicer, preview_image_path, cached_at FROM printer_models`
	var args []any
	if printerID != nil {
		query += ` WHERE printer_id = ?`
		args = append(args, *printerID)
	}
	query += ` ORDER BY modified_at IS NULL ASC, modified_at DESC, cached_at DESC, path ASC`

	rows, err := r.db.Conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing printer models: %w", err)
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var models []domain.PrinterModelFile
	for rows.Next() {
		model, err := scanPrinterModel(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning printer model: %w", err)
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func scanPrinterModel(rows *sql.Rows) (domain.PrinterModelFile, error) {
	var model domain.PrinterModelFile
	var modifiedAt sql.NullTime
	var printDurationSec int64
	err := rows.Scan(
		&model.PrinterID, &model.Path, &model.Filename, &modifiedAt, &model.SizeBytes,
		&model.EstimatedWeightG, &printDurationSec, &model.Slicer, &model.PreviewImagePath,
		&model.CachedAt,
	)
	if err != nil {
		return domain.PrinterModelFile{}, err
	}
	if modifiedAt.Valid {
		model.ModifiedAt = modifiedAt.Time
	}
	model.PrintDuration = time.Duration(printDurationSec) * time.Second
	return model, nil
}
