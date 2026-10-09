package ui

import (
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestApplicationActivatesWithAllDeclarativePages(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("GTK windows need the macOS main thread")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	spools := sqlite.NewSpoolRepo(db)
	prints := sqlite.NewPrintJobRepo(db)
	dryings := sqlite.NewDryingRepo(db)
	settings := sqlite.NewSettingsRepo(db)
	printers := sqlite.NewPrinterRepo(db)
	controller := NewApp(
		service.NewSpoolService(spools, dryings, settings),
		service.NewPrintService(prints, spools),
		service.NewDryingService(dryings, settings),
		service.NewSettingsService(settings),
		service.NewPrinterService(printers, spools, prints, settings),
	)
	application := adw.NewApplication("com.filatrack.LayoutTest", gio.ApplicationNonUnique)
	if err := application.Register(t.Context()); err != nil {
		t.Fatal(err)
	}
	controller.activate(application)
	windows := application.Windows()
	if len(windows) != 1 {
		t.Fatalf("got %d application windows", len(windows))
	}
	window := windows[0]
	defer window.Destroy()
	stack := findNavigationStack(window)
	if stack == nil {
		t.Fatal("application has no navigation stack")
	}
	for _, page := range []string{"dashboard", "spools", "prints", "models", "drying", "printers", "settings"} {
		stack.SetVisibleChildName(page)
		for range 20 {
			glib.MainContextDefault().Iteration(false)
		}
		if stack.VisibleChildName() != page || !gtk.BaseWidget(stack.VisibleChild()).IsVisible() {
			t.Fatalf("page %s did not become visible", page)
		}
	}
}

func findNavigationStack(root gtk.Widgetter) *gtk.Stack {
	if stack, ok := root.(*gtk.Stack); ok {
		return stack
	}
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		if stack := findNavigationStack(child); stack != nil {
			return stack
		}
	}
	return nil
}
