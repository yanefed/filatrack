package ui

import (
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
