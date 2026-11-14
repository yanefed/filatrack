package sqlite

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestModelMetadataMigrationAndRoundTrip(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db.Conn(), "migrations", 12); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO printers(id,name,kind,moonraker_url) VALUES('p','Printer','moonraker','http://printer')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO printer_models(printer_id,path,estimated_weight_g) VALUES('p','part.gcode',42)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	repo := NewPrinterModelRepo(db)
	models, err := repo.List(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].MetadataReady || models[0].GcodeParsed || models[0].FilamentWeightG != 0 || models[0].EstimatedWeightG != 42 {
		t.Fatalf("legacy cache: %+v", models)
	}
	model := &models[0]
	model.MetadataReady = true
	model.GcodeParsed = true
	model.SourceURL = "http://printer"
	model.FilamentLengthMM = 1000
	model.FilamentWeightG = 3
	model.ModifiedAt = time.Unix(100, 0).UTC()
	if err := repo.SaveForPrinter(t.Context(), "p", models); err != nil {
		t.Fatal(err)
	}
	restored, err := repo.List(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || !reflect.DeepEqual(restored[0], models[0]) {
		t.Fatalf("round trip: %+v", restored)
	}
	if err := goose.Down(db.Conn(), "migrations"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyReadOnlyModelSnapshotRemainsReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-snapshot.db")
	db, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db.Conn(), "migrations", 12); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO printers(id,name,kind,moonraker_url) VALUES('p','Printer','moonraker','http://printer')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO printer_models(printer_id,path,estimated_weight_g) VALUES('p','part.gcode',42)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := OpenReadOnly(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := snapshot.Close(); err != nil {
			t.Error(err)
		}
	})
	models, err := NewPrinterModelRepo(snapshot).List(t.Context(), nil)
	if err != nil || len(models) != 1 || models[0].EstimatedWeightG != 42 || models[0].MetadataReady {
		t.Fatalf("old snapshot: %+v %v", models, err)
	}
	var columns int
	if err := snapshot.Conn().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('printer_models') WHERE name='metadata_ready'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("read-only snapshot was migrated")
	}
}
