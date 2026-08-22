package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestSpoolFormPreservesExistingFields(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	materials := []domain.Material{domain.MaterialPLA, domain.MaterialPETG}
	original := domain.Spool{ID: "spool-1", Material: domain.MaterialPETG, DiameterMM: 2.85, TotalWeightG: 750}
	form := newSpoolForm(i18n.L(), materials, &original)
	updated, ok := form.Spool(materials, original)
	if !ok || updated.ID != original.ID || updated.DiameterMM != original.DiameterMM {
		t.Fatal("editing a spool must preserve fields that are not part of the form")
	}
}
