package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestDryingFormCreatesSessionForSelectedSpool(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	spools := []domain.Spool{{ID: "first"}, {ID: "second"}}
	form := newDryingForm(i18n.L())
	form.SetSpools(i18n.L(), spools, "second")
	form.tempEntry.SetText("55")
	form.durationEntry.SetText("6.5")
	session, ok := form.Session(spools)
	if !ok || session.SpoolID != "second" || session.TemperatureC != 55 || session.DurationHours != 6.5 {
		t.Fatalf("unexpected drying session: %#v", session)
	}
}
