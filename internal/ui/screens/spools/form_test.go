package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestSpoolFormLoadsExistingPhysicalProperties(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	materials := testMaterialDefinitions()
	original := domain.Spool{ID: "spool-1", Material: domain.MaterialPETG, DiameterMM: 2.85, DensityGCM3: 1.31, TotalWeightG: 750}
	form := newSpoolForm(i18n.L(), materials, &original)
	updated, ok := form.Spool(materials, original)
	if !ok || updated.ID != original.ID || updated.DiameterMM != original.DiameterMM || updated.DensityGCM3 != original.DensityGCM3 {
		t.Fatalf("physical properties were not loaded: %+v", updated)
	}
}

func TestSpoolFormAdditionalSettingsDefaults(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	materials := testMaterialDefinitions()
	form := newSpoolForm(i18n.L(), materials, nil)
	if form.additionalRow.Expanded() {
		t.Fatal("additional settings must be collapsed by default")
	}
	form.weightEntry.SetText("1000")
	spool, ok := form.Spool(materials, domain.Spool{})
	if !ok || spool.DiameterMM != 1.75 || spool.DensityGCM3 != materials[0].DensityGCM3 {
		t.Fatalf("unexpected physical property defaults: %+v", spool)
	}
	form.materialCombo.SetSelected(1)
	spool, ok = form.Spool(materials, domain.Spool{})
	if !ok || spool.DensityGCM3 != materials[1].DensityGCM3 {
		t.Fatalf("density was not updated for selected material: %+v", spool)
	}
}

func TestSpoolFormValidation(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	materials := testMaterialDefinitions()[:1]
	form := newSpoolForm(i18n.L(), materials, nil)
	if form.Valid(materials) {
		t.Fatal("empty form must be invalid")
	}
	form.weightEntry.SetText("1000")
	if !form.Valid(materials) {
		t.Fatal("positive weight must make the form valid")
	}
	form.priceEntry.SetText("invalid")
	if form.Valid(materials) {
		t.Fatal("invalid price must make the form invalid")
	}
	form.priceEntry.SetText("")
	form.diameterEntry.SetText("0")
	if form.Valid(materials) {
		t.Fatal("non-positive filament diameter must make the form invalid")
	}
	form.diameterEntry.SetText("1.75")
	form.densityEntry.SetText("invalid")
	if form.Valid(materials) {
		t.Fatal("invalid filament density must make the form invalid")
	}
}

func testMaterialDefinitions() []domain.MaterialDefinition {
	return []domain.MaterialDefinition{
		{Name: domain.MaterialPLA, DensityGCM3: 1.24},
		{Name: domain.MaterialPETG, DensityGCM3: 1.27},
	}
}
