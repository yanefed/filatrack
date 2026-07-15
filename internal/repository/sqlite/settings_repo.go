package sqlite

import (
	"context"
	"fmt"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
)

type SettingsRepo struct {
	db *DB
}

func NewSettingsRepo(db *DB) repository.SettingsRepository {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.Conn().QueryRowContext(ctx,
		`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting setting %s: %w", key, err)
	}
	return value, nil
}

func (r *SettingsRepo) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}
	return nil
}

func (r *SettingsRepo) ListDryingProfiles(ctx context.Context) ([]domain.DryingProfile, error) {
	rows, err := r.db.Conn().QueryContext(ctx,
		`SELECT material, temperature_c, duration_hours, max_days_no_dry FROM drying_profiles ORDER BY material`)
	if err != nil {
		return nil, fmt.Errorf("listing drying profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.DryingProfile
	for rows.Next() {
		var p domain.DryingProfile
		if err := rows.Scan(&p.Material, &p.TemperatureC, &p.DurationHours, &p.MaxDaysNoDry); err != nil {
			return nil, fmt.Errorf("scanning drying profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (r *SettingsRepo) UpsertDryingProfile(ctx context.Context, p *domain.DryingProfile) error {
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT INTO drying_profiles (material, temperature_c, duration_hours, max_days_no_dry)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(material) DO UPDATE SET
		   temperature_c = excluded.temperature_c,
		   duration_hours = excluded.duration_hours,
		   max_days_no_dry = excluded.max_days_no_dry`,
		string(p.Material), p.TemperatureC, p.DurationHours, p.MaxDaysNoDry)
	if err != nil {
		return fmt.Errorf("upserting drying profile: %w", err)
	}
	return nil
}

func (r *SettingsRepo) DeleteDryingProfile(ctx context.Context, material string) error {
	_, err := r.db.Conn().ExecContext(ctx,
		`DELETE FROM drying_profiles WHERE material = ?`, material)
	if err != nil {
		return fmt.Errorf("deleting drying profile: %w", err)
	}
	return nil
}

func (r *SettingsRepo) ListCustomMaterials(ctx context.Context) ([]string, error) {
	rows, err := r.db.Conn().QueryContext(ctx,
		`SELECT name FROM custom_materials ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing custom materials: %w", err)
	}
	defer rows.Close()

	var materials []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning custom material: %w", err)
		}
		materials = append(materials, name)
	}
	return materials, rows.Err()
}

func (r *SettingsRepo) AddCustomMaterial(ctx context.Context, name string) error {
	_, err := r.db.Conn().ExecContext(ctx,
		`INSERT OR IGNORE INTO custom_materials (name) VALUES (?)`, name)
	if err != nil {
		return fmt.Errorf("adding custom material: %w", err)
	}
	return nil
}

func (r *SettingsRepo) DeleteCustomMaterial(ctx context.Context, name string) error {
	_, err := r.db.Conn().ExecContext(ctx,
		`DELETE FROM custom_materials WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting custom material: %w", err)
	}
	return nil
}
