package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/widgets"
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type spoolForm struct {
	root              *adw.PreferencesGroup
	manufacturerEntry *adw.EntryRow
	materialCombo     *adw.ComboRow
	colorEntry        *adw.EntryRow
	colorPicker       *widgets.ColorPicker
	weightEntry       *adw.EntryRow
	priceEntry        *adw.EntryRow
}

func newSpoolForm(t i18n.T, materials []domain.Material, spool *domain.Spool) *spoolForm {
	form := &spoolForm{root: adw.NewPreferencesGroup()}
	form.manufacturerEntry = adw.NewEntryRow()
	form.manufacturerEntry.SetTitle(t.ManufacturerPH)
	form.root.Add(form.manufacturerEntry)

	names := make([]string, len(materials))
	selected := uint(0)
	for i, material := range materials {
		names[i] = string(material)
		if spool != nil && material == spool.Material {
			selected = uint(i)
		}
	}
	form.materialCombo = adw.NewComboRow()
	form.materialCombo.SetTitle(t.MaterialLabel)
	form.materialCombo.SetModel(gtk.NewStringList(names))
	form.materialCombo.SetSelected(selected)
	form.root.Add(form.materialCombo)

	form.colorEntry = adw.NewEntryRow()
	form.colorEntry.SetTitle(t.ColorPH)
	form.root.Add(form.colorEntry)
	colorHex := ""
	if spool != nil {
		colorHex = spool.ColorHex
	}
	form.colorPicker = widgets.NewColorPicker(t.ColorHexLabel, colorHex)
	colorRow := adw.NewActionRow()
	colorRow.SetTitle(t.ColorHexLabel)
	colorButton := form.colorPicker.Button()
	colorRow.AddSuffix(colorButton)
	colorRow.SetActivatableWidget(colorButton)
	form.root.Add(colorRow)

	form.weightEntry = adw.NewEntryRow()
	form.weightEntry.SetTitle(t.NetWeightPH)
	form.weightEntry.SetInputPurpose(gtk.InputPurposeNumber)
	form.root.Add(form.weightEntry)
	form.priceEntry = adw.NewEntryRow()
	form.priceEntry.SetTitle(t.PricePH)
	form.priceEntry.SetInputPurpose(gtk.InputPurposeNumber)
	form.root.Add(form.priceEntry)

	if spool != nil {
		form.manufacturerEntry.SetText(spool.Manufacturer)
		form.colorEntry.SetText(spool.Color)
		form.weightEntry.SetText(fmt.Sprintf("%.0f", spool.TotalWeightG))
		form.priceEntry.SetText(fmt.Sprintf("%.2f", spool.Price))
	}
	return form
}

func (f *spoolForm) Widget() *adw.PreferencesGroup {
	return f.root
}

func (f *spoolForm) Spool(materials []domain.Material, base domain.Spool) (domain.Spool, bool) {
	idx := f.materialCombo.Selected()
	if int(idx) >= len(materials) {
		return domain.Spool{}, false
	}
	weight, _ := strconv.ParseFloat(f.weightEntry.Text(), 64)
	price, _ := strconv.ParseFloat(f.priceEntry.Text(), 64)
	base.Manufacturer = f.manufacturerEntry.Text()
	base.Material = materials[idx]
	base.Color = f.colorEntry.Text()
	base.ColorHex = f.colorPicker.Hex()
	if base.DiameterMM == 0 {
		base.DiameterMM = 1.75
	}
	base.TotalWeightG = weight
	base.Price = price
	return base, true
}

func (f *spoolForm) Valid(materials []domain.Material) bool {
	if int(f.materialCombo.Selected()) >= len(materials) {
		return false
	}
	weight, err := strconv.ParseFloat(strings.TrimSpace(f.weightEntry.Text()), 64)
	if err != nil || weight <= 0 {
		return false
	}
	priceText := strings.TrimSpace(f.priceEntry.Text())
	if priceText == "" {
		return true
	}
	price, err := strconv.ParseFloat(priceText, 64)
	return err == nil && price >= 0
}
