package screens_test

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/dashboard"
	"awesomeProject/internal/ui/screens/drying"
	"awesomeProject/internal/ui/screens/models"
	"awesomeProject/internal/ui/screens/printers"
	"awesomeProject/internal/ui/screens/prints"
	"awesomeProject/internal/ui/screens/settings"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/screens/spools"
	"runtime"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	_ "modernc.org/sqlite"
)

func initGTK(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
}

func TestScreenConstructorsWithRealServices(t *testing.T) {
	initGTK(t)
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
	printerSvc := service.NewPrinterService(printerRepo, spoolRepo, printRepo, settingsRepo)
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

	dashboardPage, refreshDashboard, refreshDashboardLive := dashboard.NewDashboard(ctx, spoolSvc, printSvc, printerSvc)
	spoolsPage, refreshSpools, openAddSpoolDialog := spools.NewSpoolsScreen(ctx, nil, spoolSvc, dryingSvc, settingsSvc, func() {})
	printsPage, refreshPrints, refreshPrintsLive, refreshHistoryButton := prints.NewPrintsScreen(ctx, nil, printSvc, printerSvc, spoolSvc, func() {})
	modelsPage, refreshModels, addModelButton, refreshModelsButton := models.NewModelsScreen(ctx, nil, printerSvc, spoolSvc, settingsSvc, func() {})
	dryingPage, refreshDrying, addDryingButton := drying.NewDryingScreen(ctx, nil, dryingSvc, spoolSvc, func() {})
	printersPage, refreshPrinters, refreshPrintersLive, addPrinterButton := printers.NewPrintersScreen(ctx, nil, printerSvc, spoolSvc, func() {})
	settingsPage, refreshSettings := settings.NewSettingsScreen(ctx, nil, settingsSvc, func() {}, func() {})
	if dashboardPage == nil || spoolsPage == nil || printsPage == nil || modelsPage == nil || dryingPage == nil || printersPage == nil || settingsPage == nil {
		t.Fatal("one or more screen constructors returned nil")
	}

	refreshDashboard()
	refreshDashboardLive()
	refreshSpools()
	openAddSpoolDialog()
	refreshPrints()
	refreshPrintsLive()
	if refreshHistoryButton == nil {
		t.Fatal("prints toolbar action is nil")
	}
	if addModelButton == nil || refreshModelsButton == nil {
		t.Fatal("models toolbar actions are nil")
	}
	if addDryingButton == nil {
		t.Fatal("drying toolbar action is nil")
	}
	if addPrinterButton == nil {
		t.Fatal("printers toolbar action is nil")
	}
	refreshModels()
	refreshDrying()
	refreshPrinters()
	refreshPrintersLive()
	refreshSettings()
}

func TestModalWindowIsTransientForParent(t *testing.T) {
	initGTK(t)
	parent := gtk.NewWindow()
	dialog := shared.NewModalWindow(parent)
	t.Cleanup(parent.Destroy)
	t.Cleanup(dialog.Destroy)
	transient := dialog.TransientFor()
	if !dialog.Modal() || transient == nil || transient.Object.Native() != parent.Object.Native() {
		t.Fatal("dialog is not modal/transient for its parent")
	}
	if !dialog.DestroyWithParent() {
		t.Fatal("dialog must be destroyed with its parent")
	}
}
