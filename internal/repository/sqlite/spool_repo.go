package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"

	"github.com/google/uuid"
)

// SpoolRepo реализует repository.SpoolRepository поверх SQLite.
type SpoolRepo struct {
	db *DB
}

func NewSpoolRepo(db *DB) repository.SpoolRepository {
	return &SpoolRepo{db: db}
}

func (r *SpoolRepo) Create(ctx context.Context, spool *domain.Spool) error {
	if spool.ID == "" {
		spool.ID = uuid.New().String()
	}
	if spool.DisplayCode == "" {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, spool.ID)
	}
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO spools (id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spool.ID, spool.DisplayCode, spool.Manufacturer, string(spool.Material), spool.Color, spool.ColorHex,
		spool.DiameterMM, spool.TotalWeightG, spool.Price, spool.DensityGCM3,
		spool.PurchasedAt, boolToInt(spool.Archived), spool.CreatedAt, spool.UpdatedAt,
	)
	if err != nil && strings.Contains(err.Error(), "display_code") {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, uuid.NewString())
		_, err = r.db.Conn().ExecContext(ctx,
			`INSERT INTO spools (id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			spool.ID, spool.DisplayCode, spool.Manufacturer, string(spool.Material), spool.Color, spool.ColorHex,
			spool.DiameterMM, spool.TotalWeightG, spool.Price, spool.DensityGCM3,
			spool.PurchasedAt, boolToInt(spool.Archived), spool.CreatedAt, spool.UpdatedAt,
		)
	}
	if err != nil {
		return fmt.Errorf("inserting spool: %w", err)
	}
	return nil
}

func (r *SpoolRepo) GetByID(ctx context.Context, id string) (*domain.Spool, error) {
	row := r.db.Conn().QueryRowContext(ctx,
		`SELECT id, display_code, manufacturer, material, color, color_hex, diameter_mm, total_weight_g, price, density_g_cm3, purchased_at, archived, created_at, updated_at
		 FROM spools WHERE id = ?`, id)
	s, err := scanSpool(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("spool not found: %s", id)
		}
		return nil, fmt.Errorf("querying spool: %w", err)
	}
	return s, nil
}

func (r *SpoolRepo) List(ctx context.Context, filter repository.SpoolFilter) ([]domain.Spool, error) {
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
		args = append(args, boolToInt(*filter.Archived))
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing spools: %w", err)
	}
	defer rows.Close()

	var spools []domain.Spool
	for rows.Next() {
		s, err := scanSpoolRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning spool: %w", err)
		}
		spools = append(spools, *s)
	}
	return spools, rows.Err()
}

func (r *SpoolRepo) Update(ctx context.Context, spool *domain.Spool) error {
	spool.UpdatedAt = time.Now()
	if spool.DisplayCode == "" {
		spool.DisplayCode = domain.NewSpoolDisplayCode(spool, spool.ID)
	}
	_, err := r.db.Conn().ExecContext(ctx,
		`UPDATE spools SET display_code=?, manufacturer=?, material=?, color=?, color_hex=?, diameter_mm=?, total_weight_g=?, price=?, density_g_cm3=?, purchased_at=?, archived=?, updated_at=?
		 WHERE id=?`,
		spool.DisplayCode, spool.Manufacturer, string(spool.Material), spool.Color, spool.ColorHex,
		spool.DiameterMM, spool.TotalWeightG, spool.Price, spool.DensityGCM3,
		spool.PurchasedAt, boolToInt(spool.Archived), spool.UpdatedAt, spool.ID,
	)
	if err != nil {
		return fmt.Errorf("updating spool: %w", err)
	}
	return nil
}

func (r *SpoolRepo) Archive(ctx context.Context, id string) error {
	_, err := r.db.Conn().ExecContext(ctx,
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
	err := r.db.Conn().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(actual_weight_g), 0) FROM print_jobs WHERE spool_id = ?`, spoolID,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("getting used weight: %w", err)
	}
	return used.Float64, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanSpool(row *sql.Row) (*domain.Spool, error) {
	var s domain.Spool
	var material string
	var archived int
	var purchasedAt sql.NullTime
	err := row.Scan(&s.ID, &s.DisplayCode, &s.Manufacturer, &material, &s.Color, &s.ColorHex,
		&s.DiameterMM, &s.TotalWeightG, &s.Price, &s.DensityGCM3,
		&purchasedAt, &archived, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Material = domain.Material(material)
	s.Archived = archived != 0
	if purchasedAt.Valid {
		s.PurchasedAt = purchasedAt.Time
	}
	return &s, nil
}

func scanSpoolRows(rows *sql.Rows) (*domain.Spool, error) {
	var s domain.Spool
	var material string
	var archived int
	var purchasedAt sql.NullTime
	err := rows.Scan(&s.ID, &s.DisplayCode, &s.Manufacturer, &material, &s.Color, &s.ColorHex,
		&s.DiameterMM, &s.TotalWeightG, &s.Price, &s.DensityGCM3,
		&purchasedAt, &archived, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Material = domain.Material(material)
	s.Archived = archived != 0
	if purchasedAt.Valid {
		s.PurchasedAt = purchasedAt.Time
	}
	return &s, nil
}
