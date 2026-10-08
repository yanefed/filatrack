package sqlite

import (
	"awesomeProject/internal/port"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository/entity"

	"github.com/google/uuid"
)

// SpoolRepo реализует port.SpoolRepository поверх SQLite.
type SpoolRepo struct {
	db *DB
}

func NewSpoolRepo(db *DB) port.SpoolRepository {
	return &SpoolRepo{db: db}
}

func (r *SpoolRepo) Create(ctx context.Context, spool *domain.Spool) error {
	if spool.ID == "" {
		spool.ID = uuid.New().String()
	}
	if spool.DisplayCode == "" {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, spool.ID)
	}
	persisted := entity.SpoolFromDomain(*spool)
	_, err := r.db.conn.ExecContext(ctx,
		`INSERT INTO spools (id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		persisted.ID, persisted.DisplayCode, persisted.Manufacturer, persisted.Material, persisted.Color, persisted.ColorHex,
		persisted.DiameterMM, persisted.TotalWeightG, persisted.Price, persisted.DensityGCM3,
		persisted.PurchasedAt, persisted.Archived, persisted.CreatedAt, persisted.UpdatedAt,
	)
	if err != nil && strings.Contains(err.Error(), "display_code") {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, uuid.NewString())
		persisted = entity.SpoolFromDomain(*spool)
		_, err = r.db.conn.ExecContext(ctx,
			`INSERT INTO spools (id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			persisted.ID, persisted.DisplayCode, persisted.Manufacturer, persisted.Material, persisted.Color, persisted.ColorHex,
			persisted.DiameterMM, persisted.TotalWeightG, persisted.Price, persisted.DensityGCM3,
			persisted.PurchasedAt, persisted.Archived, persisted.CreatedAt, persisted.UpdatedAt,
		)
	}
	if err != nil {
		return fmt.Errorf("inserting spool: %w", err)
	}
	return nil
}

func (r *SpoolRepo) GetByID(ctx context.Context, id string) (*domain.Spool, error) {
	row := r.db.conn.QueryRowContext(ctx,
		`SELECT id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at
		 FROM spools WHERE id = ?`, id)
	persisted, err := entity.ScanSpool(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("spool not found: %s", id)
		}
		return nil, fmt.Errorf("querying spool: %w", err)
	}
	spool := persisted.Domain()
	return &spool, nil
}

func (r *SpoolRepo) List(ctx context.Context, filter domain.SpoolFilter) ([]domain.Spool, error) {
	query := `SELECT id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at FROM spools WHERE 1=1`
	var args []any

	if filter.Material != nil {
		query += " AND material = ?"
		args = append(args, string(*filter.Material))
	}
	if filter.Manufacturer != nil {
		query += " AND manufacturer = ?"
		args = append(args, *filter.Manufacturer)
	}
	if filter.Color != nil {
		query += " AND color = ?"
		args = append(args, *filter.Color)
	}
	if filter.Archived != nil {
		query += " AND archived = ?"
		args = append(args, entity.BoolInt(*filter.Archived))
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing spools: %w", err)
	}
	defer rows.Close()

	var spools []domain.Spool
	for rows.Next() {
		persisted, err := entity.ScanSpool(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning spool: %w", err)
		}
		spools = append(spools, persisted.Domain())
	}
	return spools, rows.Err()
}

func (r *SpoolRepo) Update(ctx context.Context, spool *domain.Spool) error {
	spool.UpdatedAt = time.Now()
	if spool.DisplayCode == "" {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, spool.ID)
	}
	persisted := entity.SpoolFromDomain(*spool)
	_, err := r.db.conn.ExecContext(ctx,
		`UPDATE spools SET display_code=?, manufacturer=?, material=?, color=?, color_hex=?, diameter_mm=?, total_weight_g=?, price=?, density_g_cm3=?, purchased_at=?, archived=?, updated_at=?
		 WHERE id=?`,
		persisted.DisplayCode, persisted.Manufacturer, persisted.Material, persisted.Color, persisted.ColorHex,
		persisted.DiameterMM, persisted.TotalWeightG, persisted.Price, persisted.DensityGCM3,
		persisted.PurchasedAt, persisted.Archived, persisted.UpdatedAt, persisted.ID,
	)
	if err != nil {
		return fmt.Errorf("updating spool: %w", err)
	}
	return nil
}

func (r *SpoolRepo) Archive(ctx context.Context, id string) error {
	_, err := r.db.conn.ExecContext(ctx,
		`UPDATE spools SET archived = 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("archiving spool: %w", err)
	}
	return nil
}

func (r *SpoolRepo) GetUsedWeight(ctx context.Context, spoolID string) (float64, error) {
	var used sql.NullFloat64
	err := r.db.conn.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(actual_weight_g), 0) FROM print_jobs WHERE spool_id = ?`, spoolID,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("getting used weight: %w", err)
	}
	return used.Float64, nil
}
