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

// PrintJobRepo реализует repository.PrintJobRepository поверх SQLite.
type PrintJobRepo struct {
	db *DB
}

func NewPrintJobRepo(db *DB) repository.PrintJobRepository {
	return &PrintJobRepo{db: db}
}

func (r *PrintJobRepo) Create(ctx context.Context, job *domain.PrintJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO print_jobs (id, spool_id, printer_id, external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, notes, printed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.SpoolID, nullString(job.PrinterID), job.ExternalPrintID, job.GcodeFilename, job.ModelName,
		string(job.Status), job.CompletionPercent,
		job.EstimatedWeightG, job.ActualWeightG, job.EstimatedLengthMM,
		int64(job.PrintDuration.Seconds()), job.NozzleTempC, job.BedTempC,
		job.Slicer, job.Notes, job.PrintedAt, job.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting print job: %w", err)
	}
	return nil
}

func (r *PrintJobRepo) GetByID(ctx context.Context, id string) (*domain.PrintJob, error) {
	row := r.db.Conn().QueryRowContext(ctx,
		`SELECT id, spool_id, COALESCE(printer_id, ''), external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, notes, printed_at, created_at
		 FROM print_jobs WHERE id = ?`, id)
	j, err := scanPrintJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("print job not found: %s", id)
		}
		return nil, fmt.Errorf("querying print job: %w", err)
	}
	return j, nil
}

func (r *PrintJobRepo) List(ctx context.Context, filter repository.PrintJobFilter) ([]domain.PrintJob, error) {
	query := `SELECT id, spool_id, COALESCE(printer_id, ''), external_print_id, gcode_filename, model_name, status, completion_percent, estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c, slicer, notes, printed_at, created_at FROM print_jobs WHERE 1=1`
	var args []any

	if filter.SpoolID != nil {
		query += " AND spool_id = ?"
		args = append(args, *filter.SpoolID)
	}
	if filter.PrinterID != nil {
		query += " AND printer_id = ?"
		args = append(args, *filter.PrinterID)
	}
	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*filter.Status))
	}
	if filter.From != nil {
		query += " AND printed_at >= ?"
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += " AND printed_at <= ?"
		args = append(args, *filter.To)
	}
	query += " ORDER BY printed_at DESC"

	rows, err := r.db.Conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing print jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.PrintJob
	for rows.Next() {
		j, err := scanPrintJobRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning print job: %w", err)
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (r *PrintJobRepo) UpdateSpool(ctx context.Context, jobID, spoolID string) error {
	result, err := r.db.Conn().ExecContext(ctx,
		`UPDATE print_jobs SET spool_id = ? WHERE id = ?`,
		spoolID, jobID,
	)
	if err != nil {
		return fmt.Errorf("updating print job spool: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("print job not found: %s", jobID)
	}
	return nil
}

func (r *PrintJobRepo) GetStats(ctx context.Context, filter repository.StatsFilter) (*repository.PrintStats, error) {
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

	var stats repository.PrintStats
	err := r.db.Conn().QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalPrints, &stats.SuccessCount, &stats.FailedCount,
		&stats.CancelledCount, &stats.TotalWeightG, &stats.AvgWeightG,
	)
	if err != nil {
		return nil, fmt.Errorf("getting print stats: %w", err)
	}
	return &stats, nil
}

func scanPrintJob(row *sql.Row) (*domain.PrintJob, error) {
	var j domain.PrintJob
	var status string
	var durationSec int64
	err := row.Scan(&j.ID, &j.SpoolID, &j.PrinterID, &j.ExternalPrintID, &j.GcodeFilename, &j.ModelName,
		&status, &j.CompletionPercent, &j.EstimatedWeightG, &j.ActualWeightG,
		&j.EstimatedLengthMM, &durationSec, &j.NozzleTempC, &j.BedTempC,
		&j.Slicer, &j.Notes, &j.PrintedAt, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	j.Status = domain.PrintStatus(status)
	j.PrintDuration = time.Duration(durationSec) * time.Second
	return &j, nil
}

func scanPrintJobRows(rows *sql.Rows) (*domain.PrintJob, error) {
	var j domain.PrintJob
	var status string
	var durationSec int64
	err := rows.Scan(&j.ID, &j.SpoolID, &j.PrinterID, &j.ExternalPrintID, &j.GcodeFilename, &j.ModelName,
		&status, &j.CompletionPercent, &j.EstimatedWeightG, &j.ActualWeightG,
		&j.EstimatedLengthMM, &durationSec, &j.NozzleTempC, &j.BedTempC,
		&j.Slicer, &j.Notes, &j.PrintedAt, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	j.Status = domain.PrintStatus(status)
	j.PrintDuration = time.Duration(durationSec) * time.Second
	return &j, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
