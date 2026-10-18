package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yanefed/filatrack/internal/config"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

// Use a real SQLite source to exercise snapshots without requiring a server.
// Reopening via New below still exercises the real PostgreSQL failure path.
func cacheSource(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := New(t.Context(), &config.Config{DataDir: dir, DBPath: filepath.Join(dir, "source.db")}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	m.selection = config.DatabaseSelection{Driver: config.DriverPostgres, PostgresURL: "postgres://filatrack@127.0.0.1:1/filatrack?sslmode=disable&connect_timeout=1"}
	m.refreshCache()
	if m.cache == nil {
		t.Fatal("cache was not initialized")
	}
	m.wrapRepositories()
	return m
}

func TestCacheTracksMutationsAndReopensReadOnly(t *testing.T) {
	m := cacheSource(t)
	ctx := t.Context()
	spool := &domain.Spool{Manufacturer: "Acme", Material: domain.MaterialPLA, Color: "Blue", TotalWeightG: 1000}
	if err := m.Spools().Create(ctx, spool); err != nil {
		t.Fatal(err)
	}
	assertCached := func(query, want string, args ...any) {
		t.Helper()
		var got string
		if err := m.cache.Conn().QueryRowContext(ctx, query, args...).Scan(&got); err != nil || got != want {
			t.Fatalf("cached query %s = %q, %v; want %q", query, got, err, want)
		}
	}
	assertCached("SELECT manufacturer FROM spools WHERE id=?", "Acme", spool.ID)
	spool.Manufacturer = "Updated"
	if err := m.Spools().Update(ctx, spool); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT manufacturer FROM spools WHERE id=?", "Updated", spool.ID)
	if err := m.Settings().SetSetting(ctx, "language", "ru"); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT value FROM app_settings WHERE key='language'", "ru")
	printer := &domain.Printer{Name: "Printer", MoonrakerURL: "http://localhost"}
	if err := m.Printers().Create(ctx, printer); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT name FROM printers WHERE id=?", "Printer", printer.ID)
	if err := m.Models().SaveForPrinter(ctx, printer.ID, []domain.PrinterModelFile{{Path: "part.gcode"}}); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT path FROM printer_models WHERE printer_id=?", "part.gcode", printer.ID)
	if err := m.Models().SaveForPrinter(ctx, printer.ID, nil); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT COUNT(*) FROM printer_models", "0")
	job := &domain.PrintJob{SpoolID: spool.ID, GcodeFilename: "part.gcode", Status: domain.PrintStatusSuccess}
	if err := m.Prints().Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := m.Prints().UpdateUsage(ctx, job.ID, 12, 11, 100, 100, true); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT actual_weight_g FROM print_jobs WHERE id=?", "11", job.ID)
	hash, err := m.PreviewImages().Put(ctx, []byte("preview"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Prints().(port.PrintJobPreviewUpdater).UpdatePreviewImageHash(ctx, job.ID, hash); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT preview_image_hash FROM print_jobs WHERE id=?", hash, job.ID)
	if err := m.Dryings().Create(ctx, &domain.DryingSession{SpoolID: spool.ID, TemperatureC: 45, DurationHours: 4}); err != nil {
		t.Fatal(err)
	}
	assertCached("SELECT COUNT(*) FROM drying_sessions", "1")
	if err := config.SaveDatabaseSelection(m.config.DataDir, m.selection); err != nil {
		t.Fatal(err)
	}
	offline, err := New(ctx, m.config, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer offline.Close()
	if !offline.ReadOnly() || offline.CacheUpdatedAt().IsZero() || offline.Selection() != m.selection {
		t.Fatal("expected read-only snapshot with timestamp and original PostgreSQL selection")
	}
	if got, err := offline.Spools().GetByID(ctx, spool.ID); err != nil || got.Manufacturer != "Updated" {
		t.Fatalf("offline spool = %+v, %v", got, err)
	}
	if got, err := offline.PreviewImages().Get(ctx, hash); err != nil || string(got) != "preview" {
		t.Fatalf("offline preview = %q, %v", got, err)
	}
	// Each mutator must reject before calling its underlying repository.
	mutations := []func() error{
		func() error { return offline.Spools().Create(ctx, nil) },
		func() error { return offline.Spools().Update(ctx, nil) },
		func() error { return offline.Spools().Archive(ctx, spool.ID) },
		func() error { return offline.Prints().Create(ctx, nil) },
		func() error { return offline.Prints().UpdateSpool(ctx, job.ID, spool.ID) },
		func() error { return offline.Prints().UpdateUsage(ctx, job.ID, 0, 0, 0, 0, false) },
		func() error {
			return offline.Prints().(port.PrintJobPreviewUpdater).UpdatePreviewImageHash(ctx, job.ID, "")
		},
		func() error { return offline.Dryings().Create(ctx, nil) },
		func() error { return offline.Printers().Create(ctx, nil) },
		func() error { return offline.Printers().Update(ctx, nil) },
		func() error { return offline.Printers().AssignSpool(ctx, printer.ID, "") },
		func() error { return offline.Printers().UpdateStatus(ctx, nil) },
		func() error { return offline.Models().SaveForPrinter(ctx, printer.ID, nil) },
		func() error { _, err := offline.PreviewImages().Put(ctx, []byte("new")); return err },
		func() error { return offline.Settings().SetSetting(ctx, "language", "bg") },
		func() error { return offline.Settings().CreateMaterial(ctx, nil) },
		func() error { return offline.Settings().UpdateMaterial(ctx, nil) },
		func() error { return offline.Settings().DeleteMaterial(ctx, domain.MaterialPLA) },
		func() error { return offline.Switch(ctx, config.DatabaseSelection{Driver: config.DriverSQLite}, true) },
	}
	for i, mutation := range mutations {
		if err := mutation(); !errors.Is(err, ErrReadOnly) {
			t.Errorf("offline mutation %d = %v", i, err)
		}
	}
	// Also enforce read-only access after database/sql reconnects.
	offline.active.conn.SetMaxIdleConns(0)
	if _, err := offline.active.conn.ExecContext(ctx, "DELETE FROM spools"); err == nil {
		t.Fatal("raw SQL write succeeded through a reopened offline connection")
	}
	selected, err := config.LoadDatabaseSelection(m.config.DataDir)
	if err != nil || selected != m.selection {
		t.Fatalf("offline launch changed selection: %+v, %v", selected, err)
	}
}

func TestOfflineRefusesMissingIncompleteOrDifferentServerCache(t *testing.T) {
	m := cacheSource(t)
	if err := m.cache.Close(); err != nil {
		t.Fatal(err)
	}
	m.cache = nil
	m.selection.PostgresURL += "&application_name=another-source"
	if err := config.SaveDatabaseSelection(m.config.DataDir, m.selection); err != nil {
		t.Fatal(err)
	}
	if opened, err := New(t.Context(), m.config, slog.Default()); err == nil {
		opened.Close()
		t.Fatal("must not use another server's snapshot")
	}
	// An initialized cache without a completed snapshot must not appear as an
	// empty application database when PostgreSQL is unavailable.
	if err := m.initializeCache(); err != nil {
		t.Fatal(err)
	}
	if opened, err := New(t.Context(), m.config, slog.Default()); err == nil {
		opened.Close()
		t.Fatal("must reject incomplete snapshot")
	}
}

func TestCacheFailureKeepsCommittedPrimaryWriteAndPreviousSnapshot(t *testing.T) {
	m := cacheSource(t)
	if _, err := m.active.conn.Exec("DROP TABLE printer_models"); err != nil {
		t.Fatal(err)
	}
	if err := m.Settings().SetSetting(context.Background(), "language", "ru"); err != nil {
		t.Fatalf("committed write must remain successful even if caching fails: %v", err)
	}
	if language, err := m.Settings().GetSetting(t.Context(), "language"); err != nil || language != "ru" {
		t.Fatalf("primary write = %q, %v", language, err)
	}
	var language string
	if err := m.cache.Conn().QueryRow("SELECT value FROM app_settings WHERE key='language'").Scan(&language); err != nil || language != "en" {
		t.Fatalf("previous snapshot = %q, %v", language, err)
	}
	if _, err := m.cache.Conn().Exec("DROP TABLE snapshot_state"); err != nil {
		t.Fatal(err)
	}
	probe := &Manager{config: m.config, selection: m.selection, logger: slog.Default()}
	if err := probe.openOffline(t.Context()); err == nil || !strings.Contains(err.Error(), "no completed snapshot") {
		t.Fatalf("incomplete cache = %v", err)
	}
}

func TestConcurrentWritesLeaveCompleteLatestSnapshot(t *testing.T) {
	m := cacheSource(t)
	var writers sync.WaitGroup
	for i := range 12 {
		writers.Go(func() {
			if err := m.Settings().SetSetting(t.Context(), fmt.Sprintf("concurrent-%d", i), "saved"); err != nil {
				t.Errorf("concurrent write: %v", err)
			}
		})
	}
	writers.Wait()
	var count int
	if err := m.cache.Conn().QueryRow("SELECT COUNT(*) FROM app_settings WHERE key LIKE 'concurrent-%' AND value='saved'").Scan(&count); err != nil || count != 12 {
		t.Fatalf("snapshot lost concurrent writes: count=%d, err=%v", count, err)
	}
}

func TestPostgresCacheRoundTrip(t *testing.T) {
	url := os.Getenv("FILATRACK_CACHE_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set FILATRACK_CACHE_TEST_POSTGRES_URL to an empty disposable PostgreSQL database")
	}
	dir := t.TempDir()
	selection := config.DatabaseSelection{Driver: config.DriverPostgres, PostgresURL: url}
	if err := config.SaveDatabaseSelection(dir, selection); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataDir: dir, DBPath: filepath.Join(dir, "unused.db")}
	m, err := New(t.Context(), cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.ReadOnly() {
		t.Fatal("expected primary PostgreSQL connection")
	}
	spool := &domain.Spool{Manufacturer: "PostgreSQL", Material: domain.MaterialPLA, Color: "Blue", TotalWeightG: 1000}
	if err := m.Spools().Create(t.Context(), spool); err != nil {
		t.Fatal(err)
	}
	if err := m.Spools().Archive(t.Context(), spool.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Settings().SetSetting(t.Context(), "language", "ru"); err != nil {
		t.Fatal(err)
	}
	offline := &Manager{config: cfg, selection: selection, logger: slog.Default()}
	if err := offline.openOffline(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer offline.Close()
	if got, err := offline.Spools().GetByID(t.Context(), spool.ID); err != nil || got.Manufacturer != "PostgreSQL" || !got.Archived {
		t.Fatalf("PostgreSQL snapshot spool = %+v, %v", got, err)
	}
	if got, err := offline.Settings().GetSetting(t.Context(), "language"); err != nil || got != "ru" {
		t.Fatalf("PostgreSQL snapshot settings = %q, %v", got, err)
	}
	offline.wrapRepositories()
	settings := offline.Settings()
	if _, err := m.active.conn.Exec("UPDATE app_settings SET value='bg' WHERE key='language'"); err != nil {
		t.Fatal(err)
	}
	if err := offline.Reconnect(t.Context()); err != nil {
		t.Fatal(err)
	}
	if offline.ReadOnly() {
		t.Fatal("successful PostgreSQL reconnect remained read-only")
	}
	if got, err := settings.GetSetting(t.Context(), "language"); err != nil || got != "bg" {
		t.Fatalf("existing handle after PostgreSQL reconnect = %q, %v", got, err)
	}
	if err := settings.SetSetting(t.Context(), "language", "en"); err != nil {
		t.Fatal(err)
	}
}
