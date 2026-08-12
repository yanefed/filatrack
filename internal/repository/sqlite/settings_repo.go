package sqlite

import (
	"context"
	"database/sql"
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

func (r *SettingsRepo) GetDryingProfileForMaterial(ctx context.Context, material domain.Material) (*domain.DryingProfile, error) {
	var profile domain.DryingProfile
	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT p.material, p.temperature_c, p.duration_hours, p.max_days_no_dry
		FROM drying_profiles p
		WHERE p.material = COALESCE(
			(SELECT drying_profile_material FROM custom_materials WHERE name = ?),
			?
		)`, string(material), string(material)).Scan(
		&profile.Material, &profile.TemperatureC, &profile.DurationHours, &profile.MaxDaysNoDry,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting drying profile for material %s: %w", material, err)
	}
	return &profile, nil
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

func (r *SettingsRepo) ListCustomMaterialBindings(ctx context.Context) ([]domain.CustomMaterial, error) {
	rows, err := r.db.Conn().QueryContext(ctx,
		`SELECT name, COALESCE(drying_profile_material, '') FROM custom_materials ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing custom material bindings: %w", err)
	}
	defer rows.Close()

	var materials []domain.CustomMaterial
	for rows.Next() {
		var material domain.CustomMaterial
		if err := rows.Scan(&material.Name, &material.DryingProfileMaterial); err != nil {
			return nil, fmt.Errorf("scanning custom material binding: %w", err)
		}
		materials = append(materials, material)
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

func (r *SettingsRepo) SetCustomMaterialDryingProfile(ctx context.Context, name, profileMaterial string) error {
	tx, err := r.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting custom material binding: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx,
		`UPDATE custom_materials SET drying_profile_material = ? WHERE name = ?`, profileMaterial, name)
	if err != nil {
		return fmt.Errorf("binding drying profile to custom material: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("checking custom material binding: %w", err)
	} else if affected == 0 {
		return fmt.Errorf("custom material %s not found", name)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM drying_profiles
		WHERE material = ?
		  AND NOT EXISTS (SELECT 1 FROM custom_materials WHERE drying_profile_material = ?)`, name, name); err != nil {
		return fmt.Errorf("deleting replaced custom drying profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing custom material binding: %w", err)
	}
	return nil
}

func (r *SettingsRepo) UpsertCustomMaterialDryingProfile(ctx context.Context, name string, profile *domain.DryingProfile) (err error) {
	tx, err := r.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting custom drying profile transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO drying_profiles (material, temperature_c, duration_hours, max_days_no_dry)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(material) DO UPDATE SET
		  temperature_c = excluded.temperature_c,
		  duration_hours = excluded.duration_hours,
		  max_days_no_dry = excluded.max_days_no_dry`,
		name, profile.TemperatureC, profile.DurationHours, profile.MaxDaysNoDry); err != nil {
		return fmt.Errorf("upserting custom drying profile: %w", err)
	}
	result, err := tx.ExecContext(ctx,
		`UPDATE custom_materials SET drying_profile_material = ? WHERE name = ?`, name, name)
	if err != nil {
		return fmt.Errorf("binding custom drying profile: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("checking custom drying profile binding: %w", err)
	} else if affected == 0 {
		return fmt.Errorf("custom material %s not found", name)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing custom drying profile: %w", err)
	}
	return nil
}

func (r *SettingsRepo) DeleteCustomMaterial(ctx context.Context, name string) error {
	tx, err := r.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting custom material deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM custom_materials WHERE name = ?`, name); err != nil {
		return fmt.Errorf("deleting custom material: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM drying_profiles
		WHERE material = ?
		  AND NOT EXISTS (SELECT 1 FROM custom_materials WHERE drying_profile_material = ?)`, name, name); err != nil {
		return fmt.Errorf("deleting unused custom drying profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing custom material deletion: %w", err)
	}
	return nil
}
