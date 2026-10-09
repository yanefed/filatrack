package database

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/yanefed/filatrack/internal/config"
	"github.com/yanefed/filatrack/internal/repository/transfer"
	_ "modernc.org/sqlite"
)

func TestSwitchCopiesDataToNewSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, DBPath: filepath.Join(dir, "source.db")}
	m, err := New(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("closing manager: %v", err)
		}
	})
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
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("closing manager: %v", err)
		}
	})
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
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("closing manager: %v", err)
		}
	})
	target := config.DatabaseSelection{Driver: config.DriverSQLite, SQLitePath: filepath.Join(dir, "target.db")}
	other, err := m.open(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.conn.Exec(`UPDATE app_settings SET value='ru' WHERE key='language'`); err != nil {
		t.Fatal(err)
	}
	if err := other.close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Switch(context.Background(), target, true); !errors.Is(err, transfer.ErrTargetNotEmpty) {
		t.Fatalf("switch = %v", err)
	}
	resumeErr := errors.New("resume failed")
	var stopped, resumed bool
	m.SetActivityControl(func() error { stopped = true; return nil }, func(context.Context) error { resumed = true; return resumeErr })
	if err := m.Switch(context.Background(), target, true); !errors.Is(err, transfer.ErrTargetNotEmpty) || !errors.Is(err, resumeErr) {
		t.Fatalf("switch must preserve transfer and resume errors: %v", err)
	}
	if !stopped || !resumed {
		t.Fatal("failed switch must resume printer activity")
	}
	selected, err := config.LoadDatabaseSelection(dir)
	if err != nil || selected.Driver != config.DriverSQLite || selected.SQLitePath != "" {
		t.Fatalf("selection changed: %+v, %v", selected, err)
	}
}
