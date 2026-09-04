package ui

import (
	"awesomeProject/internal/service"
	"runtime"
	"strings"
	"testing"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestNewAppStoresServices(t *testing.T) {
	spools := &service.SpoolService{}
	prints := &service.PrintService{}
	dryings := &service.DryingService{}
	settings := &service.SettingsService{}
	printers := &service.PrinterService{}

	app := NewApp(spools, prints, dryings, settings, printers)

	if app.SpoolService != spools || app.PrintService != prints || app.DryingService != dryings || app.SettingsService != settings || app.PrinterService != printers {
		t.Fatalf("NewApp did not retain its services: %+v", app)
	}
}

func TestSpoolToolbarButtonBecomesMapped(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}

	stack := gtk.NewStack()
	dashboard := gtk.NewBox(gtk.OrientationVertical, 0)
	spools := gtk.NewBox(gtk.OrientationVertical, 0)
	stack.AddNamed(dashboard, "dashboard")
	stack.AddNamed(spools, "spools")
	button := gtk.NewButtonFromIconName("list-add-symbolic")
	header := adw.NewHeaderBar()
	header.PackEnd(button)
	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(stack)
	window := gtk.NewWindow()
	window.SetChild(toolbar)
	window.Present()
	defer window.Destroy()

	update := func() { button.SetVisible(stackShowsPage(stack, spools)) }
	handle := stack.NotifyProperty("visible-child", update)
	defer stack.HandlerDisconnect(handle)
	stack.SetVisibleChildName("spools")
	update()
	mainContext := glib.MainContextDefault()
	for range 20 {
		mainContext.Iteration(false)
	}
	if button.Parent() == nil || !button.IsVisible() || !button.Mapped() {
		t.Fatalf("toolbar button state: parent=%v visible=%v mapped=%v", button.Parent() != nil, button.IsVisible(), button.Mapped())
	}
}

func TestStackShowsPageTracksVisibleChild(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}

	stack := gtk.NewStack()
	dashboard := gtk.NewBox(gtk.OrientationVertical, 0)
	spools := gtk.NewBox(gtk.OrientationVertical, 0)
	stack.AddNamed(dashboard, "dashboard")
	stack.AddNamed(spools, "spools")

	stack.SetVisibleChildName("dashboard")
	if stackShowsPage(stack, spools) {
		t.Fatal("spools page must not be active")
	}
	notified := false
	handle := stack.NotifyProperty("visible-child", func() { notified = true })
	defer stack.HandlerDisconnect(handle)
	stack.SetVisibleChildName("spools")
	if !notified {
		t.Fatal("switching pages must notify visible-child")
	}
	if !stackShowsPage(stack, spools) {
		t.Fatal("spools page must be active")
	}
}

func TestOpenSourceComponentsHaveAttributionMetadata(t *testing.T) {
	want := []string{"GTK", "libadwaita", "gotk4", "gotk4-adwaita", "modernc.org/sqlite", "google/uuid", "gorilla/websocket", "pressly/goose", "Uber Fx"}
	seen := make(map[string]bool, len(openSourceComponents))
	for _, component := range openSourceComponents {
		if strings.TrimSpace(component.name) == "" || strings.TrimSpace(component.license) == "" || strings.TrimSpace(component.sourceURL) == "" {
			t.Errorf("incomplete component metadata: %+v", component)
		}
		if seen[component.name] {
			t.Errorf("duplicate component %q", component.name)
		}
		seen[component.name] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Errorf("missing component %q", name)
		}
	}
}
