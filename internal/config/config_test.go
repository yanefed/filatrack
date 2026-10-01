package config

import (
	"path/filepath"
	"testing"
)

func TestDatabaseSelectionPersistsOutsideDatabase(t *testing.T) {
	dir := t.TempDir()
	choice := DatabaseSelection{Driver: DriverPostgres, PostgresURL: "postgres://localhost/filatrack?sslmode=disable"}
	if err := SaveDatabaseSelection(dir, choice); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDatabaseSelection(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != choice {
		t.Fatalf("loaded %+v, want %+v", got, choice)
	}
}

func TestDatabaseSelectionDefaultsToSQLite(t *testing.T) {
	got, err := LoadDatabaseSelection(t.TempDir())
	if err != nil || got.Driver != DriverSQLite {
		t.Fatalf("loaded %+v, %v", got, err)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.DataDir != "." {
		t.Fatalf("DataDir = %q, want .", cfg.DataDir)
	}
	if cfg.DBPath != filepath.Join(cfg.DataDir, "filatrack.db") {
		t.Fatalf("DBPath = %q, want filatrack.db in DataDir", cfg.DBPath)
	}
}
