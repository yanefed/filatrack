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

	app := NewApp(spools, prints, dryings, settings)

	if app.SpoolService != spools || app.PrintService != prints || app.DryingService != dryings || app.SettingsService != settings {
		t.Fatalf("NewApp did not retain its services: %+v", app)
	}
}
