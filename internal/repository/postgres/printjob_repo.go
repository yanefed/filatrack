package postgres

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/port"
	"awesomeProject/internal/repository/dbutil"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"awesomeProject/internal/repository/entity"

	"github.com/google/uuid"
)

// PrintJobRepo implements port.PrintJobRepository for PostgreSQL.
type PrintJobRepo struct {
	db *DB
}

func NewPrintJobRepo(db *DB) port.PrintJobRepository {
	return &PrintJobRepo{db: db}
}

func (r *PrintJobRepo) Create(ctx context.Context, job *domain.PrintJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	persisted := entity.PrintJobFromDomain(*job)
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO print_jobs (id, spool_id, printer_id, external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, preview_image_path, notes, printed_at, created_at, usage_measured, preview_image_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT (printer_id, external_print_id)
         WHERE printer_id IS NOT NULL AND external_print_id != '' DO NOTHING`,
		persisted.ID, persisted.SpoolID, persisted.PrinterID, persisted.ExternalPrintID, persisted.GcodeFilename, persisted.ModelName,
		persisted.Status, persisted.CompletionPercent,
		persisted.EstimatedWeightG, persisted.ActualWeightG, persisted.EstimatedLengthMM,
		persisted.PrintDurationSec, persisted.NozzleTempC, persisted.BedTempC,
		persisted.Slicer, persisted.PreviewImagePath, persisted.Notes, persisted.PrintedAt, persisted.CreatedAt,
		persisted.UsageMeasured, persisted.PreviewImageHash,
	)
	if err != nil {
		return fmt.Errorf("inserting print job: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking inserted print job: %w", err)
	}
	if affected == 0 {
		return domain.ErrPrintJobAlreadyExists
	}
	return nil
}

func (r *PrintJobRepo) GetByID(ctx context.Context, id string) (*domain.PrintJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, spool_id, printer_id, external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, preview_image_path, notes, printed_at, created_at, COALESCE((SELECT s.price / s.total_weight_g FROM spools s WHERE s.id = print_jobs.spool_id AND s.price > 0 AND s.total_weight_g > 0), 0), COALESCE((SELECT value FROM app_settings WHERE key = 'currency'), 'RUB'), usage_measured, preview_image_hash
		 FROM print_jobs WHERE id = ?`, id)
	persisted, err := entity.ScanPrintJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("print job not found: %s", id)
		}
		return nil, fmt.Errorf("querying print job: %w", err)
	}
	job := persisted.Domain()
	return &job, nil
}

func (r *PrintJobRepo) List(ctx context.Context, filter domain.PrintJobFilter) (result []domain.PrintJob, err error) {
	// strings.Builder writes cannot fail.
	var query strings.Builder
	_, _ = query.WriteString(`SELECT id, spool_id, printer_id, external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, preview_image_path, notes, printed_at, created_at, COALESCE((SELECT s.price / s.total_weight_g FROM spools s WHERE s.id = print_jobs.spool_id AND s.price > 0 AND s.total_weight_g > 0), 0), COALESCE((SELECT value FROM app_settings WHERE key = 'currency'), 'RUB'), usage_measured, preview_image_hash FROM print_jobs WHERE 1=1`)
	var args []any

	if filter.SpoolID != nil {
		_, _ = query.WriteString(" AND spool_id = ?")
		args = append(args, *filter.SpoolID)
	}
	if filter.PrinterID != nil {
		_, _ = query.WriteString(" AND printer_id = ?")
		args = append(args, *filter.PrinterID)
	}
	if filter.Status != nil {
		_, _ = query.WriteString(" AND status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.From != nil {
		_, _ = query.WriteString(" AND printed_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		_, _ = query.WriteString(" AND printed_at <= ?")
		args = append(args, *filter.To)
	}
	if filter.HistoryMatches != nil {
		_, _ = query.WriteString(" AND (FALSE")
		for _, match := range filter.HistoryMatches {
			if match.ExternalPrintID != "" {
				_, _ = query.WriteString(" OR external_print_id = ?")
				args = append(args, match.ExternalPrintID)
			}
			if match.Filename != "" && !match.EndedAt.IsZero() {
				minute := match.EndedAt.UTC().Truncate(time.Minute)
				_, _ = query.WriteString(" OR (gcode_filename = ? AND printed_at >= ? AND printed_at < ?)")
				args = append(args, match.Filename, minute, minute.Add(time.Minute))
			}
		}
		_, _ = query.WriteString(")")
	}
	_, _ = query.WriteString(" ORDER BY printed_at DESC")
	if filter.Limit > 0 {
		_, _ = query.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("listing print jobs: %w", err)
	}
	defer dbutil.Close(rows, &err)

	var jobs []domain.PrintJob
	for rows.Next() {
		persisted, err := entity.ScanPrintJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning print job: %w", err)
		}
		jobs = append(jobs, persisted.Domain())
	}
	return jobs, rows.Err()
}

func (r *PrintJobRepo) UpdateSpool(ctx context.Context, jobID, spoolID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE print_jobs SET spool_id = ? WHERE id = ?`,
		entity.NullString(spoolID), jobID,
	)
	if err != nil {
		return fmt.Errorf("updating print job spool: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("print job not found: %s", jobID)
	}
	return nil
}

func (r *PrintJobRepo) UpdateUsage(ctx context.Context, jobID string, estimatedWeightG, actualWeightG, estimatedLengthMM float64, completionPercent int, usageMeasured bool) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE print_jobs SET estimated_weight_g = ?, actual_weight_g = ?, estimated_length_mm = ?, completion_percent = ?, usage_measured = ? WHERE id = ?`,
		estimatedWeightG, actualWeightG, estimatedLengthMM, completionPercent, usageMeasured, jobID,
	)
	if err != nil {
		return fmt.Errorf("updating print job usage: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("print job not found: %s", jobID)
	}
	return nil
}

func (r *PrintJobRepo) UpdatePreviewImageHash(ctx context.Context, jobID, hash string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET preview_image_hash = ? WHERE id = ?`, entity.NullString(hash), jobID)
	if err != nil {
		return fmt.Errorf("updating print job preview image: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("print job not found: %s", jobID)
	}
	return nil
}

func (r *PrintJobRepo) GetStats(ctx context.Context, filter domain.StatsFilter) (*domain.PrintStats, error) {
	query := `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status='cancelled' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(actual_weight_g), 0),
		COALESCE(AVG(actual_weight_g), 0)
	FROM print_jobs WHERE 1=1`
	var args []any

	if filter.From != nil {
		query += " AND printed_at >= ?"
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += " AND printed_at <= ?"
		args = append(args, *filter.To)
	}

	var stats domain.PrintStats
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalPrints, &stats.SuccessCount, &stats.FailedCount,
		&stats.CancelledCount, &stats.TotalWeightG, &stats.AvgWeightG,
	)
	if err != nil {
		return nil, fmt.Errorf("getting print stats: %w", err)
	}
	return &stats, nil
}
