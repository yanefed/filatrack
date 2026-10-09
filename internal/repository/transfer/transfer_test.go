package transfer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/repository/postgres"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	_ "modernc.org/sqlite"
)

func newSQLite(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.New(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSQLitePostgresRoundTrip(t *testing.T) {
	url := os.Getenv("FILATRACK_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set FILATRACK_TEST_POSTGRES_URL to an empty disposable database")
	}
	ctx := context.Background()
	src := newSQLite(t)
	statements := []string{
		`INSERT INTO spools (id, display_code, manufacturer, material, color, total_weight_g) VALUES ('s1', 'S-1', 'Acme', 'PLA', 'Blue', 1000)`,
		`INSERT INTO printers (id, name, kind, moonraker_url) VALUES ('p1', 'Printer', 'moonraker', 'http://localhost')`,
		`INSERT INTO print_jobs (id, spool_id, printer_id, gcode_filename, status, estimated_weight_g, actual_weight_g) VALUES ('j1', 's1', 'p1', 'part.gcode', 'success', 10, 9)`,
		`INSERT INTO drying_sessions (id, spool_id, temperature_c, duration_hours) VALUES ('d1', 's1', 45, 4)`,
		`INSERT INTO printer_models (printer_id, path) VALUES ('p1', 'part.gcode')`,
		`UPDATE app_settings SET value = 'ru' WHERE key = 'language'`,
	}
	for _, stmt := range statements {
		if _, err := src.Conn().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	pg, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pg.Close(); err != nil {
			t.Errorf("closing PostgreSQL database: %v", err)
		}
	}()
	if err := pg.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, src.Conn(), pg.Conn(), "postgres"); err != nil {
		t.Fatal(err)
	}
	if err := Copy(ctx, src.Conn(), pg.Conn(), "postgres"); !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("copy into populated PostgreSQL = %v", err)
	}
	spools, err := pg.Spools().List(ctx, domain.SpoolFilter{})
	if err != nil || len(spools) != 1 || spools[0].Manufacturer != "Acme" {
		t.Fatalf("PostgreSQL spools = %+v, %v", spools, err)
	}
	jobs, err := pg.Prints().List(ctx, domain.PrintJobFilter{})
	if err != nil || len(jobs) != 1 || jobs[0].GcodeFilename != "part.gcode" {
		t.Fatalf("PostgreSQL jobs = %+v, %v", jobs, err)
	}
	printers, err := pg.Printers().List(ctx)
	if err != nil || len(printers) != 1 {
		t.Fatalf("PostgreSQL printers = %+v, %v", printers, err)
	}
	dryings, err := pg.Dryings(nil).List(ctx, "s1")
	if err != nil || len(dryings) != 1 {
		t.Fatalf("PostgreSQL drying = %+v, %v", dryings, err)
	}
	models, err := pg.PrinterModels().List(ctx, nil)
	if err != nil || len(models) != 1 {
		t.Fatalf("PostgreSQL models = %+v, %v", models, err)
	}
	if language, err := pg.Settings().GetSetting(ctx, "language"); err != nil || language != "ru" {
		t.Fatalf("PostgreSQL language = %q, %v", language, err)
	}
	if err := pg.Settings().SetSetting(ctx, "language", "bg"); err != nil {
		t.Fatal(err)
	}
	if err := pg.Spools().Archive(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := pg.Prints().UpdateUsage(ctx, "j1", 11, 10, 100, 100, true); err != nil {
		t.Fatal(err)
	}
	if err := pg.PrinterModels().SaveForPrinter(ctx, "p1", []domain.PrinterModelFile{{Path: "part.gcode"}}); err != nil {
		t.Fatal(err)
	}
	dst := newSQLite(t)
	if err := Copy(ctx, pg.Conn(), dst.Conn(), "sqlite"); err != nil {
		t.Fatal(err)
	}
	for _, tab := range []string{"spools", "printers", "print_jobs", "drying_sessions", "printer_models"} {
		var count int
		if err := dst.Conn().QueryRow("SELECT COUNT(*) FROM " + tab).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count = %d, %v", tab, count, err)
		}
	}
	var language string
	if err := dst.Conn().QueryRow(`SELECT value FROM app_settings WHERE key='language'`).Scan(&language); err != nil || language != "bg" {
		t.Fatalf("round-trip language = %q, %v", language, err)
	}
	var archived, measured int
	if err := dst.Conn().QueryRow(`SELECT archived FROM spools WHERE id='s1'`).Scan(&archived); err != nil || archived != 1 {
		t.Fatalf("round-trip archived = %d, %v", archived, err)
	}
	if err := dst.Conn().QueryRow(`SELECT usage_measured FROM print_jobs WHERE id='j1'`).Scan(&measured); err != nil || measured != 1 {
		t.Fatalf("round-trip usage measured = %d, %v", measured, err)
	}
	now := time.Now().UTC()
	spool := &domain.Spool{Manufacturer: "New", Material: domain.MaterialPLA, Color: "Red", ColorHex: "#ff0000", DiameterMM: 1.75, TotalWeightG: 750, DensityGCM3: 1.24, CreatedAt: now, UpdatedAt: now}
	if err := pg.Spools().Create(ctx, spool); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Spools().GetByID(ctx, spool.ID); err != nil {
		t.Fatal(err)
	}
	printer := &domain.Printer{Name: "Second", Kind: domain.PrinterKindMoonraker, MoonrakerURL: "http://localhost:7125", Enabled: true, ActiveSpoolID: spool.ID, CreatedAt: now, UpdatedAt: now}
	if err := pg.Printers().Create(ctx, printer); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Printers().GetByID(ctx, printer.ID); err != nil {
		t.Fatal(err)
	}
	job := &domain.PrintJob{SpoolID: spool.ID, PrinterID: printer.ID, GcodeFilename: "new.gcode", Status: domain.PrintStatusSuccess, EstimatedWeightG: 5, ActualWeightG: 5, PrintedAt: now, CreatedAt: now, UsageMeasured: true}
	if err := pg.Prints().Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Prints().GetByID(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	drying := &domain.DryingSession{SpoolID: spool.ID, TemperatureC: 45, DurationHours: 4, DriedAt: now, CreatedAt: now}
	if err := pg.Dryings(nil).Create(ctx, drying); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Dryings(nil).GetLastDrying(ctx, spool.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCopyPreservesDataAndRefusesOccupiedTarget(t *testing.T) {
	src, dst := newSQLite(t), newSQLite(t)
	if _, err := src.Conn().Exec(`INSERT INTO spools (id, display_code, manufacturer, material, color, total_weight_g) VALUES ('s1', 'S-1', 'Acme', 'PLA', 'Blue', 1000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Conn().Exec(`UPDATE app_settings SET value = 'ru' WHERE key = 'language'`); err != nil {
		t.Fatal(err)
	}
	if err := Copy(context.Background(), src.Conn(), dst.Conn(), "sqlite"); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := dst.Conn().QueryRow(`SELECT manufacturer FROM spools WHERE id = 's1'`).Scan(&got); err != nil || got != "Acme" {
		t.Fatalf("spool = %q, %v", got, err)
	}
	if err := dst.Conn().QueryRow(`SELECT value FROM app_settings WHERE key = 'language'`).Scan(&got); err != nil || got != "ru" {
		t.Fatalf("language = %q, %v", got, err)
	}
	if err := Copy(context.Background(), src.Conn(), dst.Conn(), "sqlite"); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("second copy = %v", err)
	}
}

func TestCopyRejectsModifiedSettingsWithoutRows(t *testing.T) {
	src, dst := newSQLite(t), newSQLite(t)
	if _, err := dst.Conn().Exec(`UPDATE app_settings SET value = 'bg' WHERE key = 'language'`); err != nil {
		t.Fatal(err)
	}
	if err := Copy(context.Background(), src.Conn(), dst.Conn(), "sqlite"); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("copy = %v", err)
	}
}

func TestCopyRollsBackWhenAnInsertedRowViolatesForeignKey(t *testing.T) {
	src, dst := newSQLite(t), newSQLite(t)
	if _, err := src.Conn().Exec(`INSERT INTO spools (id, display_code, manufacturer, material, color, total_weight_g) VALUES ('s1', 'S-1', 'Acme', 'PLA', 'Blue', 1000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Conn().Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Conn().Exec(`INSERT INTO drying_sessions (id, spool_id, temperature_c, duration_hours) VALUES ('d1', 'missing', 45, 4)`); err != nil {
		t.Fatal(err)
	}
	if err := Copy(context.Background(), src.Conn(), dst.Conn(), "sqlite"); err == nil {
		t.Fatal("expected foreign key failure")
	}
	var count int
	if err := dst.Conn().QueryRow(`SELECT COUNT(*) FROM spools`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("target retained %d spools after failure: %v", count, err)
	}
	if err := dst.Conn().QueryRow(`SELECT COUNT(*) FROM materials`).Scan(&count); err != nil || count != 7 {
		t.Fatalf("target retained %d materials after failure: %v", count, err)
	}
}
