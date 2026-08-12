package screens

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
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

	dashboard, refreshDashboard := NewDashboard(ctx, spoolSvc, printSvc)
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

func TestLargeListsAreVirtualized(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}

	const itemCount = 1000
	models := make([]domain.PrinterModelFile, itemCount)
	jobs := make([]domain.PrintJob, itemCount)
	for i := 0; i < itemCount; i++ {
		models[i] = domain.PrinterModelFile{PrinterID: "printer", Path: fmt.Sprintf("model-%d.gcode", i)}
		jobs[i] = domain.PrintJob{ID: fmt.Sprintf("job-%d", i), ModelName: fmt.Sprintf("job-%d", i)}
	}

	translations := i18n.L()
	message := gtk.NewLabel("")
	modelsView, updateModels := newVirtualModelsList(translations, context.Background(), nil, nil, message, nil)
	updateModels(models, nil, nil)
	printsView, updatePrints := newVirtualPrintsList(translations, context.Background(), nil, message)
	updatePrints(jobs, nil)

	window := gtk.NewWindow()
	window.SetDefaultSize(800, 600)
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetChild(modelsView)
	window.SetChild(scrolled)
	window.Present()
	iterateGTK()
	// A non-virtual ListBox version creates roughly ten widgets per record.
	// GtkListView should keep only a viewport-sized reusable pool.
	if widgets := widgetDescendantCount(modelsView); widgets >= itemCount*3 {
		t.Fatalf("models view created %d widgets for %d records; virtualization is not active", widgets, itemCount)
	}
	window.Destroy()

	printsWindow := gtk.NewWindow()
	printsWindow.SetDefaultSize(800, 600)
	printsScrolled := gtk.NewScrolledWindow()
	printsScrolled.SetChild(printsView)
	printsWindow.SetChild(printsScrolled)
	printsWindow.Present()
	iterateGTK()
	if widgets := widgetDescendantCount(printsView); widgets >= itemCount*3 {
		t.Fatalf("prints view created %d widgets for %d records; virtualization is not active", widgets, itemCount)
	}
	printsWindow.Destroy()
}

func iterateGTK() {
	mainContext := glib.MainContextDefault()
	for i := 0; i < 20; i++ {
		mainContext.Iteration(false)
	}
}

func widgetDescendantCount(root gtk.Widgetter) int {
	count := 0
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		count++
		count += widgetDescendantCount(child)
	}
	return count
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
