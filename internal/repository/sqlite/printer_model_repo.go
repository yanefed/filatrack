package sqlite

import (
	"context"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/repository/dbutil"
	"time"

	"github.com/yanefed/filatrack/internal/repository/entity"
)

type PrinterModelRepo struct {
	db *DB
}

func NewPrinterModelRepo(db *DB) port.PrinterModelRepository {
	return &PrinterModelRepo{db: db}
}

func (r *PrinterModelRepo) SaveForPrinter(ctx context.Context, printerID string, models []domain.PrinterModelFile) (err error) {
	tx, err := r.db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting printer models transaction: %w", err)
	}
	defer dbutil.Rollback(tx, &err)

	if _, err := tx.ExecContext(ctx, `DELETE FROM printer_models WHERE printer_id = ?`, printerID); err != nil {
		return fmt.Errorf("clearing printer models: %w", err)
	}

	now := time.Now()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO printer_models (
			printer_id, path, filename, modified_at, size_bytes, estimated_weight_g,
			print_duration_sec, slicer, preview_image_path, cached_at, preview_image_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing printer model insert: %w", err)
	}
	defer dbutil.Close(stmt, &err)

	for _, model := range models {
		model.PrinterID = printerID
		if model.CachedAt.IsZero() {
			model.CachedAt = now
		}
		persisted := entity.PrinterModelFromDomain(model)
		if _, err := stmt.ExecContext(ctx,
			persisted.PrinterID, persisted.Path, persisted.Filename, persisted.ModifiedAt, persisted.SizeBytes,
			persisted.EstimatedWeightG, persisted.PrintDurationSec, persisted.Slicer,
			persisted.PreviewImagePath, persisted.CachedAt, persisted.PreviewImageHash,
		); err != nil {
			return fmt.Errorf("inserting printer model %s: %w", model.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing printer models: %w", err)
	}
	return nil
}

func (r *PrinterModelRepo) List(ctx context.Context, printerID *string) (result []domain.PrinterModelFile, err error) {
	query := `SELECT printer_id, path, filename, modified_at, size_bytes, estimated_weight_g, print_duration_sec, slicer, preview_image_path, cached_at, preview_image_hash FROM printer_models`
	var args []any
	if printerID != nil {
		query += ` WHERE printer_id = ?`
		args = append(args, *printerID)
	}
	query += ` ORDER BY modified_at IS NULL ASC, modified_at DESC, cached_at DESC, path ASC`

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing printer models: %w", err)
	}
	defer dbutil.Close(rows, &err)

	var models []domain.PrinterModelFile
	for rows.Next() {
		persisted, err := entity.ScanPrinterModel(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning printer model: %w", err)
		}
		models = append(models, persisted.Domain())
	}
	return models, rows.Err()
}
