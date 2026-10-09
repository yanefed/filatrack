package sqlite

import (
	"github.com/yanefed/filatrack/internal/domain"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("running migrations: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})
	return db
}

func TestSpoolRepo_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSpoolRepo(db)
	ctx := t.Context()

	spool := &domain.Spool{
		Manufacturer: "Prusament",
		Material:     domain.MaterialPLA,
		Color:        "Galaxy Black",
		ColorHex:     "#1a1a2e",
		DiameterMM:   1.75,
		TotalWeightG: 1000,
		Price:        29.99,
		DensityGCM3:  1.24,
		PurchasedAt:  time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := repo.Create(ctx, spool); err != nil {
		t.Fatalf("creating spool: %v", err)
	}
	if spool.ID == "" {
		t.Fatal("expected spool ID to be set")
	}

	got, err := repo.GetByID(ctx, spool.ID)
	if err != nil {
		t.Fatalf("getting spool: %v", err)
	}
	if got.Manufacturer != "Prusament" {
		t.Errorf("expected manufacturer Prusament, got %s", got.Manufacturer)
	}
	if got.Material != domain.MaterialPLA {
		t.Errorf("expected material PLA, got %s", got.Material)
	}
}

func TestSpoolRepo_ListWithFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSpoolRepo(db)
	ctx := t.Context()

	for _, m := range []domain.Material{domain.MaterialPLA, domain.MaterialPETG} {
		spool := &domain.Spool{
			Manufacturer: "Test",
			Material:     m,
			Color:        "White",
			ColorHex:     "#ffffff",
			DiameterMM:   1.75,
			TotalWeightG: 1000,
			DensityGCM3:  1.24,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := repo.Create(ctx, spool); err != nil {
			t.Fatalf("creating spool: %v", err)
		}
	}

	mat := domain.MaterialPLA
	spools, err := repo.List(ctx, domain.SpoolFilter{Material: &mat})
	if err != nil {
		t.Fatalf("listing spools: %v", err)
	}
	if len(spools) != 1 {
		t.Errorf("expected 1 PLA spool, got %d", len(spools))
	}
}

func TestSpoolRepo_ArchiveAndUsedWeight(t *testing.T) {
	db := setupTestDB(t)
	spoolRepo := NewSpoolRepo(db)
	printRepo := NewPrintJobRepo(db)
	ctx := t.Context()

	spool := &domain.Spool{
		Manufacturer: "Test",
		Material:     domain.MaterialPLA,
		Color:        "Red",
		ColorHex:     "#ff0000",
		DiameterMM:   1.75,
		TotalWeightG: 1000,
		DensityGCM3:  1.24,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := spoolRepo.Create(ctx, spool); err != nil {
		t.Fatal(err)
	}

	job := &domain.PrintJob{
		SpoolID:           spool.ID,
		GcodeFilename:     "test.gcode",
		Status:            domain.PrintStatusSuccess,
		CompletionPercent: 100,
		EstimatedWeightG:  50,
		ActualWeightG:     50,
		PrintedAt:         time.Now(),
		CreatedAt:         time.Now(),
	}
	if err := printRepo.Create(ctx, job); err != nil {
		t.Fatal(err)
	}

	used, err := spoolRepo.GetUsedWeight(ctx, spool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if used != 50 {
		t.Errorf("expected used weight 50, got %.1f", used)
	}

	if err := spoolRepo.Archive(ctx, spool.ID); err != nil {
		t.Fatal(err)
	}
	got, err := spoolRepo.GetByID(ctx, spool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Archived {
		t.Error("expected spool to be archived")
	}
}

func TestPrintJobRepo_Stats(t *testing.T) {
	db := setupTestDB(t)
	spoolRepo := NewSpoolRepo(db)
	printRepo := NewPrintJobRepo(db)
	ctx := t.Context()

	spool := &domain.Spool{
		Manufacturer: "Test",
		Material:     domain.MaterialPLA,
		Color:        "Blue",
		ColorHex:     "#0000ff",
		DiameterMM:   1.75,
		TotalWeightG: 1000,
		DensityGCM3:  1.24,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := spoolRepo.Create(ctx, spool); err != nil {
		t.Fatal(err)
	}

	for _, s := range []domain.PrintStatus{domain.PrintStatusSuccess, domain.PrintStatusFailed, domain.PrintStatusSuccess} {
		job := &domain.PrintJob{
			SpoolID:           spool.ID,
			GcodeFilename:     "test.gcode",
			Status:            s,
			CompletionPercent: 100,
			EstimatedWeightG:  30,
			ActualWeightG:     30,
			PrintedAt:         time.Now(),
			CreatedAt:         time.Now(),
		}
		if err := printRepo.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := printRepo.GetStats(ctx, domain.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPrints != 3 {
		t.Errorf("expected 3 total prints, got %d", stats.TotalPrints)
	}
	if stats.SuccessCount != 2 {
		t.Errorf("expected 2 success, got %d", stats.SuccessCount)
	}
	if stats.FailedCount != 1 {
		t.Errorf("expected 1 failed, got %d", stats.FailedCount)
	}
}

func TestDryingRepo_CreateAndGetLast(t *testing.T) {
	db := setupTestDB(t)
	spoolRepo := NewSpoolRepo(db)
	dryingRepo := NewDryingRepo(db)
	ctx := t.Context()

	spool := &domain.Spool{
		Manufacturer: "Test",
		Material:     domain.MaterialNylon,
		Color:        "Natural",
		ColorHex:     "#f0f0f0",
		DiameterMM:   1.75,
		TotalWeightG: 750,
		DensityGCM3:  1.14,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := spoolRepo.Create(ctx, spool); err != nil {
		t.Fatal(err)
	}

	// No drying yet
	last, err := dryingRepo.GetLastDrying(ctx, spool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last != nil {
		t.Error("expected nil last drying")
	}

	session := &domain.DryingSession{
		SpoolID:       spool.ID,
		TemperatureC:  70,
		DurationHours: 6,
		DriedAt:       time.Now(),
		CreatedAt:     time.Now(),
	}
	if err := dryingRepo.Create(ctx, session); err != nil {
		t.Fatal(err)
	}

	last, err = dryingRepo.GetLastDrying(ctx, spool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last == nil {
		t.Fatal("expected non-nil last drying")
	}
	if last.TemperatureC != 70 {
		t.Errorf("expected temp 70, got %d", last.TemperatureC)
	}

	sessions, err := dryingRepo.List(ctx, spool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}
