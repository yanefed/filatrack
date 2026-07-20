package config

import (
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.DataDir != "." {
		t.Fatalf("DataDir = %q, want .", cfg.DataDir)
	}
	if cfg.DBPath != filepath.Join(cfg.DataDir, "filatrack.db") {
		t.Fatalf("DBPath = %q, want filatrack.db in DataDir", cfg.DBPath)
	}
}
