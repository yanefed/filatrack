package screens

import (
	"runtime"
	"testing"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	_ "modernc.org/sqlite"
)

func TestScreenConstructorsWithRealServices(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}

	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrating database: %v", err)
	}

	spoolRepo := sqlite.NewSpoolRepo(db)
	printRepo := sqlite.NewPrintJobRepo(db)
	dryingRepo := sqlite.NewDryingRepo(db)
	settingsRepo := sqlite.NewSettingsRepo(db)
	printerRepo := sqlite.NewPrinterRepo(db)
	spoolSvc := service.NewSpoolService(spoolRepo, dryingRepo, settingsRepo)
	printSvc := service.NewPrintService(printRepo, spoolRepo)
	dryingSvc := service.NewDryingService(dryingRepo, settingsRepo)
	settingsSvc := service.NewSettingsService(settingsRepo)
	printerSvc := service.NewPrinterService(printerRepo, spoolRepo, printRepo)
	ctx := t.Context()

	spool := &domain.Spool{Manufacturer: "UI Test", Material: domain.MaterialPLA, Color: "Black", TotalWeightG: 1000}
	if err := spoolSvc.Create(ctx, spool); err != nil {
		t.Fatalf("creating spool: %v", err)
	}
	if _, err := printSvc.CreateFromGcode(ctx, spool.ID, &domain.GcodeMetadata{Filename: "part.gcode", ModelName: "part", FilamentWeightG: 10}, domain.PrintStatusSuccess, 100); err != nil {
		t.Fatalf("creating print: %v", err)
	}
	if err := dryingSvc.Create(ctx, &domain.DryingSession{SpoolID: spool.ID, TemperatureC: 45, DurationHours: 4, DriedAt: time.Now()}); err != nil {
		t.Fatalf("creating drying session: %v", err)
	}

	dashboard, refreshDashboard := NewDashboard(ctx, spoolSvc, printSvc, dryingSvc)
	spools, refreshSpools := NewSpoolsScreen(ctx, spoolSvc, settingsSvc, func() {})
	prints, refreshPrints, refreshPrintsLive := NewPrintsScreen(ctx, printSvc, printerSvc, spoolSvc, func() {})
	models, refreshModels := NewModelsScreen(ctx, printerSvc, spoolSvc, settingsSvc, func() {})
	drying, refreshDrying := NewDryingScreen(ctx, dryingSvc, spoolSvc, func() {})
	printers, refreshPrinters, refreshPrintersLive := NewPrintersScreen(ctx, printerSvc, spoolSvc, func() {})
	settings, refreshSettings := NewSettingsScreen(ctx, settingsSvc, func() {}, func() {})
	if dashboard == nil || spools == nil || prints == nil || models == nil || drying == nil || printers == nil || settings == nil {
		t.Fatal("one or more screen constructors returned nil")
	}

	refreshDashboard()
	refreshSpools()
	refreshPrints()
	refreshPrintsLive()
	refreshModels()
	refreshDrying()
	refreshPrinters()
	refreshPrintersLive()
	refreshSettings()
	runtime.KeepAlive([]any{dashboard, spools, prints, models, drying, printers, settings})
}

func TestPrinterHasActivePrintIgnoresFinishedCurrentFile(t *testing.T) {
	if !printerHasActivePrint(domain.Printer{State: domain.PrinterStatePrinting, CurrentFile: "part.gcode"}) {
		t.Fatal("printing printer must be active")
	}
	if !printerHasActivePrint(domain.Printer{State: domain.PrinterStatePaused, CurrentFile: "part.gcode"}) {
		t.Fatal("paused printer must be active")
	}
	if printerHasActivePrint(domain.Printer{State: domain.PrinterStateReady, CurrentFile: "part.gcode", Progress: 1}) {
		t.Fatal("ready printer with stale current file must not be active")
	}
}
