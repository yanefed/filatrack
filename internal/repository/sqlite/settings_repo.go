package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/repository/dbutil"
	"github.com/yanefed/filatrack/internal/repository/entity"
)

type SettingsRepo struct {
	db *DB
}

func NewSettingsRepo(db *DB) port.SettingsRepository {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.conn.QueryRowContext(ctx,
		`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting setting %s: %w", key, err)
	}
	return value, nil
}

func (r *SettingsRepo) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.conn.ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}
	return nil
}

func (r *SettingsRepo) ListMaterials(ctx context.Context) (result []domain.MaterialDefinition, err error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT name, density_g_cm3, drying_temperature_c,
		       drying_duration_hours, max_days_no_dry
		FROM materials
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing materials: %w", err)
	}
	defer dbutil.Close(rows, &err)

	var materials []domain.MaterialDefinition
	for rows.Next() {
		persisted, err := entity.ScanMaterial(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning material: %w", err)
		}
		materials = append(materials, persisted.Domain())
	}
	return materials, rows.Err()
}

func (r *SettingsRepo) GetMaterial(ctx context.Context, name domain.Material) (*domain.MaterialDefinition, error) {
	persisted, err := entity.ScanMaterial(r.db.conn.QueryRowContext(ctx, `
		SELECT name, density_g_cm3, drying_temperature_c,
		       drying_duration_hours, max_days_no_dry
		FROM materials
		WHERE name = ?`, string(name)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting material %s: %w", name, err)
	}
	material := persisted.Domain()
	return &material, nil
}

func (r *SettingsRepo) CreateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	persisted := entity.MaterialFromDomain(*material)
	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO materials (
			name, density_g_cm3, drying_temperature_c,
			drying_duration_hours, max_days_no_dry
		) VALUES (?, ?, ?, ?, ?)`,
		persisted.Name, persisted.DensityGCM3, persisted.DryingTemperatureC,
		persisted.DryingDurationHours, persisted.MaxDaysNoDry)
	if err != nil {
		return fmt.Errorf("creating material: %w", err)
	}
	return nil
}

func (r *SettingsRepo) UpdateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	persisted := entity.MaterialFromDomain(*material)
	result, err := r.db.conn.ExecContext(ctx, `
		UPDATE materials
		SET density_g_cm3 = ?, drying_temperature_c = ?,
		    drying_duration_hours = ?, max_days_no_dry = ?
		WHERE name = ?`,
		persisted.DensityGCM3, persisted.DryingTemperatureC,
		persisted.DryingDurationHours, persisted.MaxDaysNoDry, persisted.Name)
	if err != nil {
		return fmt.Errorf("updating material: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("material %s not found", material.Name)
	}
	return nil
}

func (r *SettingsRepo) DeleteMaterial(ctx context.Context, name domain.Material) error {
	var used bool
	if err := r.db.conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM spools WHERE material = ?)`, string(name)).Scan(&used); err != nil {
		return fmt.Errorf("checking material usage: %w", err)
	}
	if used {
		return fmt.Errorf("material %s is used by one or more spools", name)
	}
	_, err := r.db.conn.ExecContext(ctx, `DELETE FROM materials WHERE name = ?`, string(name))
	if err != nil {
		return fmt.Errorf("deleting material: %w", err)
	}
	return nil
}

func (r *SettingsRepo) GetDryingProfileForMaterial(ctx context.Context, material domain.Material) (*domain.DryingProfile, error) {
	definition, err := r.GetMaterial(ctx, material)
	if err != nil {
		return nil, fmt.Errorf("getting drying profile for material %s: %w", material, err)
	}
	if definition == nil {
		return nil, nil
	}
	profile := definition.DryingProfile()
	return &profile, nil
}
