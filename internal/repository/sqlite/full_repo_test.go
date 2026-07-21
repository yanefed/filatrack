package sqlite

import (
	"context"
	"strings"
	"testing"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
)

func testSpool(id string, createdAt time.Time) *domain.Spool {
	return &domain.Spool{
		ID: id, Manufacturer: "Maker", Material: domain.MaterialPLA, Color: "Black",
		ColorHex: "#000000", DiameterMM: 1.75, TotalWeightG: 1000, Price: 25,
		DensityGCM3: 1.24, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
}

func TestSpoolRepoFullCRUDAndFilters(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSpoolRepo(db)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)

	first := testSpool("spool-1", now.Add(-time.Hour))
	first.PurchasedAt = now.Add(-24 * time.Hour)
	second := testSpool("spool-2", now)
	second.Manufacturer = "Other"
	second.Material = domain.MaterialPETG
	second.Color = "Blue"
	second.Archived = true
	for _, spool := range []*domain.Spool{first, second} {
		if err := repo.Create(ctx, spool); err != nil {
			t.Fatalf("creating spool %s: %v", spool.ID, err)
		}
	}

	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("getting spool: %v", err)
	}
	if got.ID != first.ID || got.DisplayCode == "" || got.PurchasedAt.IsZero() || got.Archived {
		t.Fatalf("unexpected spool: %+v", got)
	}
	if _, err := repo.GetByID(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing spool error = %v", err)
	}

	manufacturer, color, archived, material := "Other", "Blue", true, domain.MaterialPETG
	filtered, err := repo.List(ctx, repository.SpoolFilter{
		Manufacturer: &manufacturer, Color: &color, Archived: &archived, Material: &material,
	})
	if err != nil || len(filtered) != 1 || filtered[0].ID != second.ID {
		t.Fatalf("filtered spools = (%+v, %v)", filtered, err)
	}
	active := false
	filtered, err = repo.List(ctx, repository.SpoolFilter{Archived: &active})
	if err != nil || len(filtered) != 1 || filtered[0].ID != first.ID {
		t.Fatalf("active spools = (%+v, %v)", filtered, err)
	}

	first.Manufacturer = "Updated"
	first.Color = "White"
	first.ColorHex = "#ffffff"
	first.TotalWeightG = 750
	first.Archived = true
	beforeUpdate := time.Now()
	if err := repo.Update(ctx, first); err != nil {
		t.Fatalf("updating spool: %v", err)
	}
	if first.UpdatedAt.Before(beforeUpdate) {
		t.Fatal("Update did not refresh UpdatedAt")
	}
	got, err = repo.GetByID(ctx, first.ID)
	if err != nil || got.Manufacturer != "Updated" || got.TotalWeightG != 750 || got.DisplayCode == "" || !got.Archived {
		t.Fatalf("updated spool = (%+v, %v)", got, err)
	}

	if err := repo.Create(ctx, first); err == nil || !strings.Contains(err.Error(), "inserting spool") {
		t.Fatalf("duplicate spool error = %v", err)
	}
}

func TestSpoolRepoDatabaseErrors(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSpoolRepo(db)
	if err := db.Close(); err != nil {
		t.Fatalf("closing database: %v", err)
	}
	ctx := t.Context()
	spool := testSpool("spool", time.Now())

	checks := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error { return repo.Create(ctx, spool) }},
		{name: "get", run: func() error { _, err := repo.GetByID(ctx, spool.ID); return err }},
		{name: "list", run: func() error { _, err := repo.List(ctx, repository.SpoolFilter{}); return err }},
		{name: "update", run: func() error { return repo.Update(ctx, spool) }},
		{name: "archive", run: func() error { return repo.Archive(ctx, spool.ID) }},
		{name: "used weight", run: func() error { _, err := repo.GetUsedWeight(ctx, spool.ID); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("expected closed-database error")
			}
		})
	}
}

func TestPrintJobRepoFullCRUDFiltersAndStats(t *testing.T) {
	db := setupTestDB(t)
	spools := NewSpoolRepo(db)
	prints := NewPrintJobRepo(db)
	ctx := t.Context()
	base := time.Now().UTC().Truncate(time.Second)
	for _, spool := range []*domain.Spool{testSpool("spool-1", base), testSpool("spool-2", base)} {
		if err := spools.Create(ctx, spool); err != nil {
			t.Fatalf("creating spool: %v", err)
		}
	}
	printers := NewPrinterRepo(db)
	if err := printers.Create(ctx, &domain.Printer{
		ID:           "printer-1",
		Name:         "Voron",
		Kind:         domain.PrinterKindMoonraker,
		MoonrakerURL: "http://printer.local",
		Enabled:      true,
		CreatedAt:    base,
		UpdatedAt:    base,
	}); err != nil {
		t.Fatalf("creating printer: %v", err)
	}

	jobs := []*domain.PrintJob{
		{ID: "job-1", SpoolID: "spool-1", PrinterID: "printer-1", ExternalPrintID: "moonraker-job-1", GcodeFilename: "one.gcode", ModelName: "one", Status: domain.PrintStatusSuccess, CompletionPercent: 100, EstimatedWeightG: 20, ActualWeightG: 20, EstimatedLengthMM: 1000, PrintDuration: 90 * time.Minute, NozzleTempC: 210, BedTempC: 60, Slicer: "PrusaSlicer", PreviewImagePath: "/tmp/one.png", Notes: "ok", PrintedAt: base.Add(-2 * time.Hour), CreatedAt: base},
		{ID: "job-2", SpoolID: "spool-1", GcodeFilename: "two.gcode", Status: domain.PrintStatusFailed, CompletionPercent: 50, EstimatedWeightG: 20, ActualWeightG: 10, PrintedAt: base.Add(-time.Hour), CreatedAt: base},
		{ID: "job-3", SpoolID: "spool-2", GcodeFilename: "three.gcode", Status: domain.PrintStatusCancelled, CompletionPercent: 25, EstimatedWeightG: 20, ActualWeightG: 5, PrintedAt: base, CreatedAt: base},
		{ID: "job-4", PrinterID: "printer-1", ExternalPrintID: "moonraker-job-4", GcodeFilename: "backfill.gcode", ModelName: "backfill", Status: domain.PrintStatusSuccess, CompletionPercent: 100, EstimatedWeightG: 20, ActualWeightG: 0, PrintedAt: base.Add(-3 * time.Hour), CreatedAt: base},
	}
	for _, job := range jobs {
		if err := prints.Create(ctx, job); err != nil {
			t.Fatalf("creating job %s: %v", job.ID, err)
		}
	}

	got, err := prints.GetByID(ctx, "job-1")
	if err != nil || got.ModelName != "one" || got.PrinterID != "printer-1" || got.ExternalPrintID != "moonraker-job-1" || got.PrintDuration != 90*time.Minute || got.Status != domain.PrintStatusSuccess || got.PreviewImagePath != "/tmp/one.png" {
		t.Fatalf("GetByID() = (%+v, %v)", got, err)
	}
	if _, err := prints.GetByID(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing print error = %v", err)
	}

	spoolID, status := "spool-1", domain.PrintStatusFailed
	from, to := base.Add(-90*time.Minute), base.Add(-30*time.Minute)
	filtered, err := prints.List(ctx, repository.PrintJobFilter{SpoolID: &spoolID, Status: &status, From: &from, To: &to})
	if err != nil || len(filtered) != 1 || filtered[0].ID != "job-2" {
		t.Fatalf("filtered jobs = (%+v, %v)", filtered, err)
	}
	printerID := "printer-1"
	filtered, err = prints.List(ctx, repository.PrintJobFilter{PrinterID: &printerID})
	if err != nil || len(filtered) != 2 || filtered[0].ID != "job-1" || filtered[1].ID != "job-4" {
		t.Fatalf("printer-filtered jobs = (%+v, %v)", filtered, err)
	}
	all, err := prints.List(ctx, repository.PrintJobFilter{})
	if err != nil || len(all) != 4 || all[0].ID != "job-3" {
		t.Fatalf("ordered jobs = (%+v, %v)", all, err)
	}
	limited, err := prints.List(ctx, repository.PrintJobFilter{Limit: 2})
	if err != nil || len(limited) != 2 || limited[0].ID != "job-3" || limited[1].ID != "job-2" {
		t.Fatalf("limited jobs = (%+v, %v)", limited, err)
	}
	got, err = prints.GetByID(ctx, "job-4")
	if err != nil || got.SpoolID != "" || got.ActualWeightG != 0 {
		t.Fatalf("nullable spool job = (%+v, %v)", got, err)
	}

	stats, err := prints.GetStats(ctx, repository.StatsFilter{})
	if err != nil || stats.TotalPrints != 4 || stats.SuccessCount != 2 || stats.FailedCount != 1 || stats.CancelledCount != 1 || stats.TotalWeightG != 35 {
		t.Fatalf("all stats = (%+v, %v)", stats, err)
	}
	stats, err = prints.GetStats(ctx, repository.StatsFilter{From: &from, To: &to})
	if err != nil || stats.TotalPrints != 1 || stats.FailedCount != 1 || stats.AvgWeightG != 10 {
		t.Fatalf("filtered stats = (%+v, %v)", stats, err)
	}

	duplicate := *jobs[0]
	if err := prints.Create(ctx, &duplicate); err == nil {
		t.Fatal("expected duplicate print ID error")
	}
	invalid := *jobs[0]
	invalid.ID = "bad-fk"
	invalid.SpoolID = "missing"
	if err := prints.Create(ctx, &invalid); err == nil {
		t.Fatal("expected foreign-key error")
	}

	if err := prints.UpdateSpool(ctx, "job-1", "spool-2"); err != nil {
		t.Fatalf("updating print spool: %v", err)
	}
	got, err = prints.GetByID(ctx, "job-1")
	if err != nil || got.SpoolID != "spool-2" {
		t.Fatalf("updated print spool = (%+v, %v)", got, err)
	}
}

func TestPrintJobRepoDatabaseErrors(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPrintJobRepo(db)
	if err := db.Close(); err != nil {
		t.Fatalf("closing database: %v", err)
	}
	ctx := t.Context()
	job := &domain.PrintJob{ID: "job"}
	checks := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error { return repo.Create(ctx, job) }},
		{name: "get", run: func() error { _, err := repo.GetByID(ctx, job.ID); return err }},
		{name: "list", run: func() error { _, err := repo.List(ctx, repository.PrintJobFilter{}); return err }},
		{name: "stats", run: func() error { _, err := repo.GetStats(ctx, repository.StatsFilter{}); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("expected closed-database error")
			}
		})
	}
}

func TestPrinterRepoCRUDAndAssignments(t *testing.T) {
	db := setupTestDB(t)
	spools := NewSpoolRepo(db)
	repo := NewPrinterRepo(db)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)

	if err := spools.Create(ctx, testSpool("spool-1", now)); err != nil {
		t.Fatalf("creating spool: %v", err)
	}
	printer := &domain.Printer{
		ID:           "printer-1",
		Name:         "Voron",
		Kind:         domain.PrinterKindMoonraker,
		MoonrakerURL: "http://voron.local",
		APIKey:       "secret",
		Enabled:      true,
		State:        domain.PrinterStateUnknown,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.Create(ctx, printer); err != nil {
		t.Fatalf("creating printer: %v", err)
	}

	got, err := repo.GetByID(ctx, printer.ID)
	if err != nil || got.Name != "Voron" || got.Kind != domain.PrinterKindMoonraker || !got.Enabled {
		t.Fatalf("printer = (%+v, %v)", got, err)
	}
	listed, err := repo.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != printer.ID {
		t.Fatalf("listed printers = (%+v, %v)", listed, err)
	}

	if err := repo.AssignSpool(ctx, printer.ID, "spool-1"); err != nil {
		t.Fatalf("assigning spool: %v", err)
	}
	got, err = repo.GetByID(ctx, printer.ID)
	if err != nil || got.ActiveSpoolID != "spool-1" {
		t.Fatalf("assigned printer = (%+v, %v)", got, err)
	}

	got.State = domain.PrinterStatePrinting
	got.CurrentFile = "benchy.gcode"
	got.Progress = 0.42
	got.LastSeenAt = now.Add(time.Minute)
	if err := repo.UpdateStatus(ctx, got); err != nil {
		t.Fatalf("updating status: %v", err)
	}
	got, err = repo.GetByID(ctx, printer.ID)
	if err != nil || got.State != domain.PrinterStatePrinting || got.CurrentFile != "benchy.gcode" || got.Progress != 0.42 || got.LastSeenAt.IsZero() {
		t.Fatalf("status printer = (%+v, %v)", got, err)
	}

	got.Name = "Trident"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("updating printer: %v", err)
	}
	got, err = repo.GetByID(ctx, printer.ID)
	if err != nil || got.Name != "Trident" {
		t.Fatalf("updated printer = (%+v, %v)", got, err)
	}

	if err := repo.AssignSpool(ctx, printer.ID, "missing"); err == nil {
		t.Fatal("expected missing spool foreign-key error")
	}
	if _, err := repo.GetByID(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing printer error = %v", err)
	}
}

func TestPrinterRepoDatabaseErrors(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPrinterRepo(db)
	if err := db.Close(); err != nil {
		t.Fatalf("closing database: %v", err)
	}
	ctx := context.Background()
	printer := &domain.Printer{ID: "printer"}
	checks := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error { return repo.Create(ctx, printer) }},
		{name: "get", run: func() error { _, err := repo.GetByID(ctx, "printer"); return err }},
		{name: "list", run: func() error { _, err := repo.List(ctx); return err }},
		{name: "update", run: func() error { return repo.Update(ctx, printer) }},
		{name: "assign", run: func() error { return repo.AssignSpool(ctx, "printer", "spool") }},
		{name: "status", run: func() error { return repo.UpdateStatus(ctx, printer) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("expected closed-database error")
			}
		})
	}
}

func TestPrinterModelRepoCache(t *testing.T) {
	db := setupTestDB(t)
	printers := NewPrinterRepo(db)
	models := NewPrinterModelRepo(db)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)

	if err := printers.Create(ctx, &domain.Printer{
		ID:           "printer-1",
		Name:         "Voron",
		Kind:         domain.PrinterKindMoonraker,
		MoonrakerURL: "http://voron.local",
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("creating printer: %v", err)
	}
	if err := printers.Create(ctx, &domain.Printer{
		ID:           "printer-2",
		Name:         "Trident",
		Kind:         domain.PrinterKindMoonraker,
		MoonrakerURL: "http://trident.local",
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("creating second printer: %v", err)
	}

	first := []domain.PrinterModelFile{
		{PrinterID: "ignored", Path: "folder/cube.gcode", Filename: "cube.gcode", ModifiedAt: now.Add(-time.Hour), SizeBytes: 1234, EstimatedWeightG: 12.5, PrintDuration: 30 * time.Minute, Slicer: "OrcaSlicer", PreviewImagePath: "/tmp/cube.png"},
		{Path: "benchy.gcode", Filename: "benchy.gcode", SizeBytes: 456},
	}
	if err := models.SaveForPrinter(ctx, "printer-1", first); err != nil {
		t.Fatalf("saving printer models: %v", err)
	}
	if err := models.SaveForPrinter(ctx, "printer-2", []domain.PrinterModelFile{{Path: "other.gcode", Filename: "other.gcode"}}); err != nil {
		t.Fatalf("saving second printer models: %v", err)
	}

	printerID := "printer-1"
	listed, err := models.List(ctx, &printerID)
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed printer models = (%+v, %v)", listed, err)
	}
	if listed[0].PrinterID != "printer-1" || listed[0].Path != "folder/cube.gcode" || listed[0].EstimatedWeightG != 12.5 || listed[0].PrintDuration != 30*time.Minute || listed[0].PreviewImagePath != "/tmp/cube.png" || listed[0].CachedAt.IsZero() {
		t.Fatalf("cached model = %+v", listed[0])
	}

	if err := models.SaveForPrinter(ctx, "printer-1", []domain.PrinterModelFile{{Path: "new.gcode", Filename: "new.gcode"}}); err != nil {
		t.Fatalf("replacing printer models: %v", err)
	}
	listed, err = models.List(ctx, &printerID)
	if err != nil || len(listed) != 1 || listed[0].Path != "new.gcode" {
		t.Fatalf("replaced printer models = (%+v, %v)", listed, err)
	}
	all, err := models.List(ctx, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("all printer models = (%+v, %v)", all, err)
	}
}

func TestDryingRepoOrderingAndErrors(t *testing.T) {
	db := setupTestDB(t)
	spools := NewSpoolRepo(db)
	repo := NewDryingRepo(db)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)
	if err := spools.Create(ctx, testSpool("spool", now)); err != nil {
		t.Fatalf("creating spool: %v", err)
	}

	for _, session := range []*domain.DryingSession{
		{ID: "old", SpoolID: "spool", TemperatureC: 50, DurationHours: 4, DriedAt: now.Add(-time.Hour), CreatedAt: now},
		{ID: "new", SpoolID: "spool", TemperatureC: 55, DurationHours: 5, DriedAt: now, CreatedAt: now},
	} {
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("creating drying session: %v", err)
		}
	}
	sessions, err := repo.List(ctx, "spool")
	if err != nil || len(sessions) != 2 || sessions[0].ID != "new" {
		t.Fatalf("ordered drying sessions = (%+v, %v)", sessions, err)
	}
	last, err := repo.GetLastDrying(ctx, "spool")
	if err != nil || last == nil || last.ID != "new" {
		t.Fatalf("last drying = (%+v, %v)", last, err)
	}
	invalid := &domain.DryingSession{ID: "invalid", SpoolID: "missing", TemperatureC: 50, DurationHours: 1, DriedAt: now, CreatedAt: now}
	if err := repo.Create(ctx, invalid); err == nil {
		t.Fatal("expected foreign-key error")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("closing database: %v", err)
	}
	if _, err := repo.List(ctx, "spool"); err == nil {
		t.Fatal("expected list error on closed database")
	}
	if _, err := repo.GetLastDrying(ctx, "spool"); err == nil {
		t.Fatal("expected last-drying error on closed database")
	}
	if err := repo.Create(ctx, &domain.DryingSession{}); err == nil {
		t.Fatal("expected create error on closed database")
	}
}

func TestSettingsRepoCRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	ctx := t.Context()

	if value, err := repo.GetSetting(ctx, "language"); err != nil || value != "en" {
		t.Fatalf("default language = (%q, %v)", value, err)
	}
	if err := repo.SetSetting(ctx, "language", "bg"); err != nil {
		t.Fatalf("updating language: %v", err)
	}
	if value, err := repo.GetSetting(ctx, "language"); err != nil || value != "bg" {
		t.Fatalf("updated language = (%q, %v)", value, err)
	}
	if _, err := repo.GetSetting(ctx, "missing"); err == nil {
		t.Fatal("expected missing setting error")
	}

	profiles, err := repo.ListDryingProfiles(ctx)
	if err != nil || len(profiles) != len(domain.DefaultDryingProfiles) {
		t.Fatalf("default profiles = (%d, %v)", len(profiles), err)
	}
	customProfile := &domain.DryingProfile{Material: "Wood", TemperatureC: 45, DurationHours: 8, MaxDaysNoDry: 2}
	if err := repo.UpsertDryingProfile(ctx, customProfile); err != nil {
		t.Fatalf("inserting profile: %v", err)
	}
	customProfile.TemperatureC = 50
	if err := repo.UpsertDryingProfile(ctx, customProfile); err != nil {
		t.Fatalf("updating profile: %v", err)
	}
	if err := repo.DeleteDryingProfile(ctx, string(customProfile.Material)); err != nil {
		t.Fatalf("deleting profile: %v", err)
	}

	for _, name := range []string{"Wood", "HIPS", "Wood"} {
		if err := repo.AddCustomMaterial(ctx, name); err != nil {
			t.Fatalf("adding material %q: %v", name, err)
		}
	}
	materials, err := repo.ListCustomMaterials(ctx)
	if err != nil || len(materials) != 2 || materials[0] != "HIPS" || materials[1] != "Wood" {
		t.Fatalf("custom materials = (%+v, %v)", materials, err)
	}
	if err := repo.DeleteCustomMaterial(ctx, "Wood"); err != nil {
		t.Fatalf("deleting material: %v", err)
	}
	materials, err = repo.ListCustomMaterials(ctx)
	if err != nil || len(materials) != 1 || materials[0] != "HIPS" {
		t.Fatalf("materials after delete = (%+v, %v)", materials, err)
	}
}

func TestSettingsRepoDatabaseErrors(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	if err := db.Close(); err != nil {
		t.Fatalf("closing database: %v", err)
	}
	ctx := context.Background()
	profile := &domain.DryingProfile{}
	checks := []struct {
		name string
		run  func() error
	}{
		{name: "get setting", run: func() error { _, err := repo.GetSetting(ctx, "key"); return err }},
		{name: "set setting", run: func() error { return repo.SetSetting(ctx, "key", "value") }},
		{name: "list profiles", run: func() error { _, err := repo.ListDryingProfiles(ctx); return err }},
		{name: "upsert profile", run: func() error { return repo.UpsertDryingProfile(ctx, profile) }},
		{name: "delete profile", run: func() error { return repo.DeleteDryingProfile(ctx, "PLA") }},
		{name: "list materials", run: func() error { _, err := repo.ListCustomMaterials(ctx); return err }},
		{name: "add material", run: func() error { return repo.AddCustomMaterial(ctx, "Wood") }},
		{name: "delete material", run: func() error { return repo.DeleteCustomMaterial(ctx, "Wood") }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("expected closed-database error")
			}
		})
	}
}

func TestMigrateReturnsSchemaError(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Conn().Exec("CREATE TABLE spools (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("creating conflicting table: %v", err)
	}
	if err := db.Migrate(); err == nil || !strings.Contains(err.Error(), "running migrations") {
		t.Fatalf("migration error = %v", err)
	}
}

func TestNewReturnsOpenError(t *testing.T) {
	if db, err := New(t.TempDir()); err == nil {
		_ = db.Close()
		t.Fatal("expected opening a directory as SQLite database to fail")
	}
}
