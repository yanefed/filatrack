package widgets

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"math"
	"runtime"
	"strings"
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

func TestPrinterHasActivePrint(t *testing.T) {
	if !PrinterHasActivePrint(domain.Printer{State: domain.PrinterStatePrinting, CurrentFile: "part.gcode"}) {
		t.Fatal("printing printer must be active")
	}
	if !PrinterHasActivePrint(domain.Printer{State: domain.PrinterStatePaused, CurrentFile: "part.gcode"}) {
		t.Fatal("paused printer must be active")
	}
	if PrinterHasActivePrint(domain.Printer{State: domain.PrinterStateReady, CurrentFile: "part.gcode", Progress: 1}) {
		t.Fatal("ready printer with stale current file must not be active")
	}
}

func TestPrinterInfoTextOnlyIncludesProgressForActivePrint(t *testing.T) {
	translations := i18n.L()
	idle := PrinterInfoText(translations, domain.Printer{Name: "Voron", State: domain.PrinterStateReady, Progress: 0.75})
	if strings.Contains(idle, "%") {
		t.Fatalf("idle printer status contains progress: %q", idle)
	}
	printing := PrinterInfoText(translations, domain.Printer{Name: "Voron", State: domain.PrinterStatePrinting, Progress: 0.75})
	if !strings.Contains(printing, "75%") {
		t.Fatalf("printing printer status does not contain progress: %q", printing)
	}
}

func TestScreenLayout(t *testing.T) {
	initGTK(t)

	screen := NewScreen("Inventory")
	if screen.MarginTop() != screenMargin || screen.MarginBottom() != screenMargin ||
		screen.MarginStart() != screenMargin || screen.MarginEnd() != screenMargin {
		t.Fatalf("screen margins must all equal %d", screenMargin)
	}
	child := screen.FirstChild()
	title, ok := child.(*gtk.Label)
	if !ok || title.Text() != "Inventory" || !title.HasCSSClass("title-1") {
		t.Fatal("screen must start with a title-1 label")
	}
}

func TestClampProgress(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  float64
	}{{-1, 0}, {0.4, 0.4}, {2, 1}} {
		if got := ClampProgress(tt.value); got != tt.want {
			t.Fatalf("ClampProgress(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestColorPickerHex(t *testing.T) {
	initGTK(t)

	picker := NewColorPicker("Filament color", "#1a2b3c")
	if got := picker.Hex(); got != "#1a2b3c" {
		t.Fatalf("Hex() = %q, want #1a2b3c", got)
	}

	picker.SetHex("#abcdef")
	if got := picker.Hex(); got != "#abcdef" {
		t.Fatalf("Hex() after SetHex() = %q, want #abcdef", got)
	}
	if picker.Widget() == nil {
		t.Fatal("Widget() returned nil")
	}
}

func TestSpoolPreviewProperties(t *testing.T) {
	initGTK(t)

	preview := NewSpoolPreview("#123456")
	if preview.ContentWidth() != 58 || preview.ContentHeight() != 58 {
		t.Fatalf("preview size = %dx%d, want 58x58", preview.ContentWidth(), preview.ContentHeight())
	}
	if got := preview.TooltipText(); got != "#123456" {
		t.Fatalf("tooltip = %q, want #123456", got)
	}
}

func TestLevelColor(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		want     [3]float64
	}{
		{name: "empty is red", fraction: 0, want: [3]float64{0.88, 0.11, 0.14}},
		{name: "half is yellow", fraction: 0.5, want: [3]float64{0.96, 0.76, 0.07}},
		{name: "full is green", fraction: 1, want: [3]float64{0.18, 0.76, 0.49}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			red, green, blue := levelColor(tt.fraction)
			got := [3]float64{red, green, blue}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 0.001 {
					t.Fatalf("levelColor(%v) = %v, want %v", tt.fraction, got, tt.want)
				}
			}
		})
	}
}

func TestLevelProgressProperties(t *testing.T) {
	initGTK(t)

	progress := NewLevelProgress(42, "42% remaining")
	if progress.ContentWidth() != levelProgressWidth || progress.ContentHeight() != levelProgressHeight {
		t.Fatalf("progress size = %dx%d, want %dx%d", progress.ContentWidth(), progress.ContentHeight(), levelProgressWidth, levelProgressHeight)
	}
	if got := progress.TooltipText(); got != "42% remaining" {
		t.Fatalf("tooltip = %q, want 42%% remaining", got)
	}
}
