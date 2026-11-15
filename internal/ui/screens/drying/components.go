package drying

import (
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
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

func newDryingForm(t i18n.T, views ...*layout.View) *dryingForm {
	view := layout.Use("drying-form", views...)

	form := &dryingForm{root: layout.Get[*adw.PreferencesGroup](view, "form_root")}
	form.spoolDropdown = layout.Get[*adw.ComboRow](view, "form_spoolDropdown")
	form.spoolDropdown.SetTitle(t.SpoolColon)
	form.spoolDropdown.SetModel(gtk.NewStringList([]string{t.NoSpoolsAvail}))

	form.tempEntry = layout.Get[*adw.EntryRow](view, "form_tempEntry")
	form.tempEntry.SetTitle(t.TemperatureC)

	form.durationEntry = layout.Get[*adw.EntryRow](view, "form_durationEntry")
	form.durationEntry.SetTitle(t.DurationHours)

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
	if !f.Valid(spools) {
		return nil, false
	}
	idx := f.spoolDropdown.Selected()
	if int(idx) >= len(spools) {
		return nil, false
	}
	temperature, _ := strconv.Atoi(strings.TrimSpace(f.tempEntry.Text()))
	duration, _ := shared.ParseDecimal(f.durationEntry.Text())
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
	duration, err := shared.ParseDecimal(strings.TrimSpace(f.durationEntry.Text()))
	return err == nil && duration > 0
}

func (f *dryingForm) Validate(t i18n.T, spools []domain.Spool, focus bool) bool {
	temp, te := strconv.Atoi(strings.TrimSpace(f.tempEntry.Text()))
	duration, de := shared.ParseDecimal(f.durationEntry.Text())
	tempOK, durationOK := te == nil && temp > 0, de == nil && duration > 0
	shared.FieldError(f.tempEntry, t.TemperatureC, t.InvalidPositive, tempOK)
	shared.FieldError(f.durationEntry, t.DurationHours, t.InvalidPositive, durationOK)
	spoolOK := int(f.spoolDropdown.Selected()) < len(spools)
	f.spoolDropdown.SetSubtitle("")
	if !spoolOK {
		f.spoolDropdown.SetSubtitle(t.RequiredField)
	}
	if focus {
		if !spoolOK {
			f.spoolDropdown.GrabFocus()
		} else if !tempOK {
			f.tempEntry.GrabFocus()
		} else if !durationOK {
			f.durationEntry.GrabFocus()
		}
	}
	return spoolOK && tempOK && durationOK
}
