package transfer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrTargetNotEmpty = errors.New("target database is not empty")

type table struct{ name, columns string }

var tables = []table{
	{"materials", "name,density_g_cm3,drying_temperature_c,drying_duration_hours,max_days_no_dry"},
	{"app_settings", "key,value"},
	{"spools", "id,display_code,manufacturer,material,color,color_hex,diameter_mm,total_weight_g,price,density_g_cm3,purchased_at,archived,created_at,updated_at"},
	{"printers", "id,name,kind,moonraker_url,api_key,enabled,active_spool_id,state,current_file,progress,last_seen_at,created_at,updated_at"},
	{"preview_images", "hash,image_data,created_at"},
	{"print_jobs", "id,spool_id,printer_id,external_print_id,gcode_filename,model_name,status,completion_percent,estimated_weight_g,actual_weight_g,estimated_length_mm,print_duration_sec,nozzle_temp_c,bed_temp_c,slicer,preview_image_path,notes,printed_at,created_at,usage_measured,preview_image_hash"},
	{"drying_sessions", "id,spool_id,temperature_c,duration_hours,dried_at,created_at"},
	{"printer_models", "printer_id,path,filename,modified_at,size_bytes,estimated_weight_g,print_duration_sec,slicer,preview_image_path,cached_at,preview_image_hash"},
}

// Copy copies a complete application database into a freshly initialized
// database. It never changes the source and commits the destination only after
// every table has been copied successfully.
func Copy(ctx context.Context, source, target *sql.DB, targetDriver string) error {
	return copyDatabase(ctx, source, target, targetDriver, false)
}

// ReplaceSQLite atomically refreshes a local snapshot. A failed refresh leaves
// the previous snapshot intact, including its completion marker.
func ReplaceSQLite(ctx context.Context, source, target *sql.DB) error {
	return copyDatabase(ctx, source, target, "sqlite", true)
}

func copyDatabase(ctx context.Context, source, target *sql.DB, targetDriver string, replace bool) error {
	if source == target {
		return fmt.Errorf("source and target databases are identical")
	}
	if targetDriver != "sqlite" && targetDriver != "postgres" {
		return fmt.Errorf("unsupported target driver %q", targetDriver)
	}
	src, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("starting source snapshot: %w", err)
	}
	defer src.Rollback()
	dst, err := target.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting target transaction: %w", err)
	}
	defer dst.Rollback()
	if targetDriver == "postgres" {
		if _, err := dst.ExecContext(ctx, "LOCK TABLE materials, app_settings, spools, printers, preview_images, print_jobs, drying_sessions, printer_models IN ACCESS EXCLUSIVE MODE"); err != nil {
			return fmt.Errorf("locking target tables: %w", err)
		}
	}
	if replace {
		// Delete dependent rows first so foreign key enforcement stays enabled.
		for i := len(tables) - 1; i >= 0; i-- {
			if _, err := dst.ExecContext(ctx, "DELETE FROM "+tables[i].name); err != nil {
				return fmt.Errorf("clearing snapshot %s: %w", tables[i].name, err)
			}
		}
	} else {
		if err := checkEmpty(ctx, dst); err != nil {
			return err
		}
		for _, name := range []string{"materials", "app_settings"} {
			if _, err := dst.ExecContext(ctx, "DELETE FROM "+name); err != nil {
				return fmt.Errorf("clearing default %s: %w", name, err)
			}
		}
	}
	for _, tab := range tables {
		if err := copyTable(ctx, src, dst, tab, targetDriver); err != nil {
			return fmt.Errorf("copying %s: %w", tab.name, err)
		}
	}
	if replace {
		if _, err := dst.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS snapshot_state (id INTEGER PRIMARY KEY CHECK (id=1), updated_at TEXT NOT NULL)"); err != nil {
			return err
		}
		if _, err := dst.ExecContext(ctx, "INSERT OR REPLACE INTO snapshot_state (id, updated_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))"); err != nil {
			return err
		}
	}
	if err := dst.Commit(); err != nil {
		return fmt.Errorf("committing target database: %w", err)
	}
	return nil
}

func checkEmpty(ctx context.Context, tx *sql.Tx) error {
	for _, name := range []string{"spools", "printers", "preview_images", "print_jobs", "drying_sessions", "printer_models"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+name).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("%w: %s has %d rows", ErrTargetNotEmpty, name, count)
		}
	}
	defaults := map[string]string{"language": "en", "low_filament_level": "50", "currency": "RUB"}
	rows, err := tx.QueryContext(ctx, "SELECT key, value FROM app_settings")
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			_ = rows.Close()
			return err
		}
		if expected, ok := defaults[key]; !ok || value != expected {
			_ = rows.Close()
			return fmt.Errorf("%w: settings were changed", ErrTargetNotEmpty)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	if count != len(defaults) {
		return fmt.Errorf("%w: settings differ from defaults", ErrTargetNotEmpty)
	}
	var materialCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM materials WHERE
		(name='PLA' AND density_g_cm3=1.24 AND drying_temperature_c=45 AND drying_duration_hours=4 AND max_days_no_dry=30) OR
		(name='PETG' AND density_g_cm3=1.27 AND drying_temperature_c=65 AND drying_duration_hours=4 AND max_days_no_dry=14) OR
		(name='ABS' AND density_g_cm3=1.04 AND drying_temperature_c=60 AND drying_duration_hours=4 AND max_days_no_dry=30) OR
		(name='ASA' AND density_g_cm3=1.07 AND drying_temperature_c=60 AND drying_duration_hours=4 AND max_days_no_dry=30) OR
		(name='TPU' AND density_g_cm3=1.21 AND drying_temperature_c=50 AND drying_duration_hours=5 AND max_days_no_dry=7) OR
		(name='Nylon' AND density_g_cm3=1.14 AND drying_temperature_c=70 AND drying_duration_hours=6 AND max_days_no_dry=7) OR
		(name='PC' AND density_g_cm3=1.20 AND drying_temperature_c=70 AND drying_duration_hours=8 AND max_days_no_dry=7)`).Scan(&materialCount); err != nil {
		return err
	}
	var totalMaterials int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM materials").Scan(&totalMaterials); err != nil {
		return err
	}
	if materialCount != 7 || totalMaterials != 7 {
		return fmt.Errorf("%w: materials were changed", ErrTargetNotEmpty)
	}
	return nil
}

func copyTable(ctx context.Context, src, dst *sql.Tx, tab table, driver string) error {
	columns := strings.Split(tab.columns, ",")
	rows, err := src.QueryContext(ctx, "SELECT "+tab.columns+" FROM "+tab.name)
	if err != nil {
		return err
	}
	defer rows.Close()
	marks := make([]string, len(columns))
	for i := range marks {
		if driver == "postgres" {
			marks[i] = fmt.Sprintf("$%d", i+1)
		} else {
			marks[i] = "?"
		}
	}
	stmt, err := dst.PrepareContext(ctx, "INSERT INTO "+tab.name+" ("+tab.columns+") VALUES ("+strings.Join(marks, ",")+")")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return err
		}
	}
	return rows.Err()
}
