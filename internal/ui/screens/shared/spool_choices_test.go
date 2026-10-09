package shared

import (
	"github.com/yanefed/filatrack/internal/domain"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestSelectedSpoolIndexUsesUnassignedOption(t *testing.T) {
	spools := []domain.Spool{{ID: "first"}, {ID: "second"}}
	for _, tc := range []struct {
		name    string
		spoolID string
		want    uint
	}{
		{name: "unassigned after another choice", spoolID: "", want: 0},
		{name: "archived spool absent from choices", spoolID: "missing", want: 0},
		{name: "assigned spool", spoolID: "second", want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectedSpoolIndex(tc.spoolID, spools); got != tc.want {
				t.Fatalf("SelectedSpoolIndex(%q) = %d, want %d", tc.spoolID, got, tc.want)
			}
		})
	}
}

func TestReusedSpoolDropDownClearsPreviousSelection(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("GTK widgets cannot be presented from Go's test goroutine on macOS")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	spools := []domain.Spool{{ID: "first"}, {ID: "second"}}
	dropdown := gtk.NewDropDown(gtk.NewStringList([]string{"Unassigned", "First", "Second"}), nil)
	dropdown.SetSelected(SelectedSpoolIndex("second", spools))
	dropdown.SetSelected(SelectedSpoolIndex("", spools))
	if got := dropdown.Selected(); got != 0 {
		t.Fatalf("selection after switching to unassigned print = %d, want 0", got)
	}
}
