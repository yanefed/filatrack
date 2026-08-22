package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type dryingForm struct {
	root          *adw.PreferencesGroup
	spoolDropdown *adw.ComboRow
	tempEntry     *adw.EntryRow
	durationEntry *adw.EntryRow
}

func newDryingForm(t i18n.T) *dryingForm {
	form := &dryingForm{root: adw.NewPreferencesGroup()}
	form.spoolDropdown = adw.NewComboRow()
	form.spoolDropdown.SetTitle(t.SpoolColon)
	form.spoolDropdown.SetModel(gtk.NewStringList([]string{t.NoSpoolsAvail}))
	form.root.Add(form.spoolDropdown)
	form.tempEntry = adw.NewEntryRow()
	form.tempEntry.SetTitle(t.TemperatureC)
	form.tempEntry.SetInputPurpose(gtk.InputPurposeNumber)
	form.root.Add(form.tempEntry)
	form.durationEntry = adw.NewEntryRow()
	form.durationEntry.SetTitle(t.DurationHours)
	form.durationEntry.SetInputPurpose(gtk.InputPurposeNumber)
	form.root.Add(form.durationEntry)
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

func (f *dryingForm) SetRecommendation(profile *domain.DryingProfile) {
	if profile == nil {
		f.tempEntry.SetText("")
		f.durationEntry.SetText("")
		return
	}
	f.tempEntry.SetText(strconv.Itoa(profile.TemperatureC))
	f.durationEntry.SetText(strconv.FormatFloat(profile.DurationHours, 'f', -1, 64))
}

func (f *dryingForm) Valid(spools []domain.Spool) bool {
	if int(f.spoolDropdown.Selected()) >= len(spools) {
		return false
	}
	temperature, err := strconv.Atoi(strings.TrimSpace(f.tempEntry.Text()))
	if err != nil || temperature <= 0 {
		return false
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(f.durationEntry.Text()), 64)
	return err == nil && duration > 0
}
