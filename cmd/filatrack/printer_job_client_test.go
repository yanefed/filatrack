package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/yanefed/filatrack/internal/config"
	"github.com/yanefed/filatrack/internal/database"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/moonraker"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/repository/transfer"
)

func TestPrinterCommandsRemainBlockedWhileOffline(t *testing.T) {
	dir := t.TempDir()
	selection := config.DatabaseSelection{Driver: config.DriverPostgres, PostgresURL: "postgres://filatrack@127.0.0.1:1/filatrack?sslmode=disable"}
	if err := config.SaveDatabaseSelection(dir, selection); err != nil {
		t.Fatal(err)
	}
	key := sha256.Sum256([]byte(selection.PostgresURL))
	cacheDir := filepath.Join(dir, "cache", "database")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := sqlite.New(filepath.Join(cacheDir, fmt.Sprintf("%x.db", key)))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	source, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for _, db := range []*sqlite.DB{source, snapshot} {
		if err := db.Migrate(); err != nil {
			t.Fatal(err)
		}
	}
	if err := transfer.ReplaceSQLite(t.Context(), source.Conn(), snapshot.Conn()); err != nil {
		t.Fatal(err)
	}
	db, err := database.New(t.Context(), &config.Config{DataDir: dir}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := databaseAwareJobClient{HTTPControlClient: moonraker.HTTPControlClient{}, database: db}
	// Invalid printer/path would fail differently if either command reached
	// the HTTP client. The offline guard must run before any remote side effect.
	if err := client.StartFile(t.Context(), domain.Printer{}, "part.gcode"); !errors.Is(err, database.ErrReadOnly) {
		t.Fatalf("offline start = %v", err)
	}
	if _, err := client.Upload(t.Context(), domain.Printer{}, "missing.gcode", "part.gcode", true); !errors.Is(err, database.ErrReadOnly) {
		t.Fatalf("offline upload = %v", err)
	}
}
