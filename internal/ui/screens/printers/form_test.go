package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func initGTK(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
}

func TestPrinterFormPreservesExistingFields(t *testing.T) {
	initGTK(t)
	original := domain.Printer{ID: "printer-1", Name: "Voron", MoonrakerURL: "http://voron.local", APIKey: "secret", Enabled: true}
	form := newPrinterForm(i18n.L(), &original)
	updated := form.Printer(original)
	if updated.ID != original.ID || updated.Enabled != original.Enabled || updated.APIKey != original.APIKey {
		t.Fatalf("printer fields were not preserved: %+v", updated)
	}
	if !form.Valid() {
		t.Fatal("complete printer form must be valid")
	}
}

func TestPrinterFormRequiresNameAndHTTPURL(t *testing.T) {
	initGTK(t)
	form := newPrinterForm(i18n.L(), nil)
	form.nameEntry.SetText("Printer")
	form.urlEntry.SetText("printer.local")
	if form.Valid() {
		t.Fatal("URL without an HTTP scheme must be invalid")
	}
	form.urlEntry.SetText("http://printer.local")
	if !form.Valid() {
		t.Fatal("name and HTTP URL must make the form valid")
	}
}
