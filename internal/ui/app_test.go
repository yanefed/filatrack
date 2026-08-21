package ui

import (
	"strings"
	"testing"

	"awesomeProject/internal/service"
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
