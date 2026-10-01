package database

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"awesomeProject/internal/config"
	"awesomeProject/internal/repository/transfer"
	_ "modernc.org/sqlite"
)

func TestSwitchCopiesDataToNewSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, DBPath: filepath.Join(dir, "source.db")}
	m, err := New(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.active.conn.Exec(`INSERT INTO spools (id, display_code, manufacturer, material, color, total_weight_g) VALUES ('s1', 'S-1', 'Acme', 'PLA', 'Blue', 1000)`); err != nil {
		t.Fatal(err)
	}
	target := config.DatabaseSelection{Driver: config.DriverSQLite, SQLitePath: filepath.Join(dir, "target.db")}
	if err := m.Switch(context.Background(), target, true); err != nil {
		t.Fatal(err)
	}
	selected, err := config.LoadDatabaseSelection(dir)
	if err != nil || selected != target {
		t.Fatalf("selection = %+v, %v", selected, err)
	}
	reopened, err := New(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.active.conn.QueryRow(`SELECT COUNT(*) FROM spools WHERE id = 's1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("target spools = %d, %v", count, err)
	}
}

func TestSwitchRejectsOccupiedTargetWithoutChangingSelection(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, DBPath: filepath.Join(dir, "source.db")}
	m, err := New(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	target := config.DatabaseSelection{Driver: config.DriverSQLite, SQLitePath: filepath.Join(dir, "target.db")}
	other, err := m.open(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.conn.Exec(`UPDATE app_settings SET value='ru' WHERE key='language'`); err != nil {
		t.Fatal(err)
	}
	_ = other.close()
	if err := m.Switch(context.Background(), target, true); !errors.Is(err, transfer.ErrTargetNotEmpty) {
		t.Fatalf("switch = %v", err)
	}
	selected, err := config.LoadDatabaseSelection(dir)
	if err != nil || selected.Driver != config.DriverSQLite || selected.SQLitePath != "" {
		t.Fatalf("selection changed: %+v, %v", selected, err)
	}
}
