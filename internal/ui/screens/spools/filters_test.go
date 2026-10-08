package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestSpoolFiltersSelectionAndRefresh(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	changes := 0
	filters := newSpoolFilters(i18n.L(), func() { changes++ })
	options := service.SpoolFilterOptions{Manufacturers: []string{"", "B"}, Materials: []domain.Material{domain.MaterialPLA, "PLA+"}}
	filters.SetOptions(options)
	filters.manufacturer.SetSelected(2)
	filters.material.SetSelected(2)
	filter := filters.Filter()
	if filter.Manufacturer == nil || *filter.Manufacturer != "B" || filter.Material == nil || *filter.Material != "PLA+" || filter.Archived == nil || *filter.Archived {
		t.Fatalf("unexpected selection: %+v", filter)
	}
	if changes != 2 {
		t.Fatalf("expected two changes, got %d", changes)
	}
	options.Manufacturers = []string{"", "A", "B"}
	filters.SetOptions(options)
	filter = filters.Filter()
	if changes != 2 || filter.Manufacturer == nil || *filter.Manufacturer != "B" || filters.manufacturer.Selected() != 3 {
		t.Fatal("refresh must preserve values without emitting changes")
	}
	options.Manufacturers = []string{"", "A"}
	filters.SetOptions(options)
	if filters.Filter().Manufacturer != nil || filters.Filter().Material == nil {
		t.Fatal("only the vanished selection should reset")
	}
	filters.manufacturer.SetSelected(1)
	if value := filters.Filter().Manufacturer; value == nil || *value != "" {
		t.Fatal("unspecified manufacturer must remain distinct from all manufacturers")
	}
	beforeReset := changes
	filters.reset.Emit("clicked")
	filter = filters.Filter()
	if filter.Material != nil || filter.Manufacturer != nil || filters.reset.Sensitive() || changes != beforeReset+1 {
		t.Fatal("reset must clear both filters and emit one change")
	}
	filters.SetOptions(service.SpoolFilterOptions{})
	if filters.Filter().Material != nil || filters.Filter().Manufacturer != nil {
		t.Fatal("empty options must leave filters unrestricted")
	}
}
