package dashboard

import (
	"github.com/yanefed/filatrack/internal/domain"
	"testing"
)

func TestDashboardDataSummaries(t *testing.T) {
	data := dashboardData{
		spools: []dashboardSpool{
			{Spool: domain.Spool{ID: "one"}, RemainingG: 400, NeedsDrying: true},
			{Spool: domain.Spool{ID: "two"}, RemainingG: 600},
		},
		printers: []domain.Printer{
			{ID: "idle", State: domain.PrinterStateReady},
			{ID: "active", State: domain.PrinterStatePrinting},
		},
	}
	if data.totalRemainingG() != 1000 {
		t.Fatalf("remaining filament = %v", data.totalRemainingG())
	}
	if warnings := data.dryingWarnings(); len(warnings) != 1 || warnings[0].ID != "one" {
		t.Fatalf("drying warnings = %+v", warnings)
	}
	if active := data.activePrinters(); len(active) != 1 || active[0].ID != "active" {
		t.Fatalf("active printers = %+v", active)
	}
}

func TestFormatFilamentWeight(t *testing.T) {
	if got := formatFilamentWeight(750); got != "750 g" {
		t.Fatalf("grams = %q", got)
	}
	if got := formatFilamentWeight(1250); got != "1.25 kg" {
		t.Fatalf("kilograms = %q", got)
	}
}
