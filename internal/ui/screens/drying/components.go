package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type dryingForm struct {
	root          *gtk.Box
	spoolDropdown *gtk.DropDown
	tempEntry     *gtk.Entry
	durationEntry *gtk.Entry
}

func newDryingForm(t i18n.T) *dryingForm {
	form := &dryingForm{root: gtk.NewBox(gtk.OrientationVertical, 6)}
	form.root.SetVisible(false)
	form.spoolDropdown = gtk.NewDropDown(gtk.NewStringList([]string{t.NoSpoolsAvail}), nil)
	form.root.Append(form.spoolDropdown)
	form.tempEntry = gtk.NewEntry()
	form.tempEntry.SetPlaceholderText(t.TemperatureC)
	form.root.Append(form.tempEntry)
	form.durationEntry = gtk.NewEntry()
	form.durationEntry.SetPlaceholderText(t.DurationHours)
	form.root.Append(form.durationEntry)
	return form
}

func (f *dryingForm) SetSpools(t i18n.T, spools []domain.Spool, currentID string) {
	names := make([]string, len(spools))
	selected := uint(0)
	for i, spool := range spools {
		names[i] = fmt.Sprintf("%s %s (%s)", spool.Manufacturer, spool.Color, spool.Material)
		if spool.ID == currentID {
			selected = uint(i)
		}
	}
	if len(names) == 0 {
		names = []string{t.NoSpoolsAvail}
	}
	f.spoolDropdown.SetModel(gtk.NewStringList(names))
	f.spoolDropdown.SetSelected(selected)
}

func (f *dryingForm) Session(spools []domain.Spool) (*domain.DryingSession, bool) {
	idx := f.spoolDropdown.Selected()
	if int(idx) >= len(spools) {
		return nil, false
	}
	temperature, _ := strconv.Atoi(f.tempEntry.Text())
	duration, _ := strconv.ParseFloat(f.durationEntry.Text(), 64)
	return &domain.DryingSession{SpoolID: spools[idx].ID, TemperatureC: temperature, DurationHours: duration}, true
}

func (f *dryingForm) Reset() {
	f.tempEntry.SetText("")
	f.durationEntry.SetText("")
}
