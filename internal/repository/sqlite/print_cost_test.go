package sqlite

import (
	"awesomeProject/internal/domain"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestPrintCostUsesCurrentSpoolPrice(t *testing.T) {
	db := setupTestDB(t)
	ctx := t.Context()
	spools := NewSpoolRepo(db)
	prints := NewPrintJobRepo(db)
	settings := NewSettingsRepo(db)
	spool := &domain.Spool{Manufacturer: "Test", Material: domain.MaterialPLA, Color: "White", TotalWeightG: 1000, Price: 2000, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := spools.Create(ctx, spool); err != nil {
		t.Fatal(err)
	}
	for _, status := range []domain.PrintStatus{domain.PrintStatusSuccess, domain.PrintStatusCancelled, domain.PrintStatusFailed} {
		job := &domain.PrintJob{SpoolID: spool.ID, Status: status, GcodeFilename: "part.gcode", ActualWeightG: 25, UsageMeasured: true, PrintedAt: time.Now(), CreatedAt: time.Now()}
		if err := prints.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	spool.Price = 3000
	if err := spools.Update(ctx, spool); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetSetting(ctx, "currency", "EUR"); err != nil {
		t.Fatal(err)
	}
	jobs, err := prints.List(ctx, domain.PrintJobFilter{})
	if err != nil || len(jobs) != 3 {
		t.Fatalf("jobs: %+v, %v", jobs, err)
	}
	for _, job := range jobs {
		if job.PricePerGram != 3 || job.CostCurrency != "EUR" || job.CostIsEstimate() {
			t.Fatalf("current price: %+v", job)
		}
		if err := prints.UpdateSpool(ctx, job.ID, spool.ID); err != nil {
			t.Fatal(err)
		}
		if err := prints.UpdateUsage(ctx, job.ID, 30, 4, 1000, 10, true); err != nil {
			t.Fatal(err)
		}
		loaded, err := prints.GetByID(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if amount, _ := loaded.FilamentCost(); amount != 12 || loaded.CostCurrency != "EUR" {
			t.Fatalf("usage update: %+v", loaded)
		}
	}
	job := &domain.PrintJob{SpoolID: spool.ID, Status: domain.PrintStatusCancelled, GcodeFilename: "new.gcode", ActualWeightG: 4, PrintedAt: time.Now(), CreatedAt: time.Now()}
	if err := prints.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	job, err = prints.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if amount, _ := job.FilamentCost(); amount != 12 || job.CostCurrency != "EUR" || !job.CostIsEstimate() {
		t.Fatalf("new print: %+v", job)
	}
	if err := prints.UpdateSpool(ctx, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err := prints.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.FilamentCost(); ok {
		t.Fatal("unassigned print has cost")
	}
	if err := prints.UpdateSpool(ctx, job.ID, spool.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = prints.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if amount, ok := loaded.FilamentCost(); !ok || amount != 12 {
		t.Fatalf("reassigned print: %+v", loaded)
	}
}

func TestPrintCostMigrationUsesDynamicPrices(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db.Conn(), "migrations", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO spools (id, manufacturer, material, color, total_weight_g, price) VALUES ('spool', 'Test', 'PLA', 'White', 1000, 2000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO print_jobs (id, spool_id, gcode_filename, status, estimated_weight_g, actual_weight_g) VALUES ('old', 'spool', 'part.gcode', 'cancelled', 25, 4), ('unknown', NULL, 'other.gcode', 'failed', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	var storedPriceColumns int
	if err := db.Conn().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('print_jobs') WHERE name IN ('price_per_gram', 'cost_currency', 'price_estimated')`).Scan(&storedPriceColumns); err != nil {
		t.Fatal(err)
	}
	if storedPriceColumns != 0 {
		t.Fatal("price snapshots still stored")
	}
	job, err := NewPrintJobRepo(db).GetByID(t.Context(), "old")
	if err != nil {
		t.Fatal(err)
	}
	if amount, ok := job.FilamentCost(); !ok || amount != 8 || !job.CostIsEstimate() {
		t.Fatalf("dynamic price: %+v", job)
	}
	unknown, err := NewPrintJobRepo(db).GetByID(t.Context(), "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unknown.FilamentCost(); ok {
		t.Fatal("unknown price was invented")
	}
	if err := goose.Down(db.Conn(), "migrations"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
}
