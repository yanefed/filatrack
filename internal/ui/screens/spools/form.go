package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/layout"
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
	additionalRow     *adw.ExpanderRow
	diameterEntry     *adw.EntryRow
	densityEntry      *adw.EntryRow
}

func newSpoolForm(t i18n.T, materials []domain.MaterialDefinition, spool *domain.Spool, views ...*layout.View) *spoolForm {
	builder := layout.Use("spool-form", views...)
	form := &spoolForm{
		root:              layout.Get[*adw.PreferencesGroup](builder, "spool_form"),
		manufacturerEntry: layout.Get[*adw.EntryRow](builder, "manufacturer_entry"),
		materialCombo:     layout.Get[*adw.ComboRow](builder, "material_combo"),
		colorEntry:        layout.Get[*adw.EntryRow](builder, "color_entry"),
		weightEntry:       layout.Get[*adw.EntryRow](builder, "weight_entry"),
		priceEntry:        layout.Get[*adw.EntryRow](builder, "price_entry"),
		additionalRow:     layout.Get[*adw.ExpanderRow](builder, "additional_row"),
		diameterEntry:     layout.Get[*adw.EntryRow](builder, "diameter_entry"),
		densityEntry:      layout.Get[*adw.EntryRow](builder, "density_entry"),
	}
	form.manufacturerEntry.SetTitle(t.ManufacturerPH)

	names := make([]string, len(materials))
	selected := uint(0)
	for i, material := range materials {
		names[i] = string(material.Name)
		if spool != nil && material.Name == spool.Material {
			selected = uint(i)
		} else if spool == nil && material.Name == domain.MaterialPLA {
			selected = uint(i)
		}
	}
	form.materialCombo.SetTitle(t.MaterialLabel)
	form.materialCombo.SetModel(gtk.NewStringList(names))
	form.materialCombo.SetSelected(selected)

	form.colorEntry.SetTitle(t.ColorPH)
	colorHex := ""
	if spool != nil {
		colorHex = spool.ColorHex
	}
	form.colorPicker = widgets.NewColorPicker(t.ColorHexLabel, colorHex, layout.Scope(builder, "color_picker"))
	colorRow := layout.Get[*adw.ActionRow](builder, "color_row")
	colorRow.SetTitle(t.ColorHexLabel)

	form.weightEntry.SetTitle(t.NetWeightPH)
	form.priceEntry.SetTitle(t.PricePH)
	form.additionalRow.SetTitle(t.AdditionalSettings)
	form.diameterEntry.SetTitle(t.FilamentDiameter)
	form.densityEntry.SetTitle(t.FilamentDensity)

	diameter := domain.DefaultDiameter
	density := selectedMaterialDensity(materials, selected)

	if spool != nil {
		form.manufacturerEntry.SetText(spool.Manufacturer)
		form.colorEntry.SetText(spool.Color)
		form.weightEntry.SetText(fmt.Sprintf("%.0f", spool.TotalWeightG))
		form.priceEntry.SetText(fmt.Sprintf("%.2f", spool.Price))
		if spool.DiameterMM > 0 {
			diameter = spool.DiameterMM
		}
		if spool.DensityGCM3 > 0 {
			density = spool.DensityGCM3
		}
	}
	form.diameterEntry.SetText(strconv.FormatFloat(diameter, 'f', -1, 64))
	form.densityEntry.SetText(strconv.FormatFloat(density, 'f', -1, 64))
	form.materialCombo.NotifyProperty("selected", func() {
		form.densityEntry.SetText(strconv.FormatFloat(selectedMaterialDensity(materials, form.materialCombo.Selected()), 'f', -1, 64))
	})
	return form
}

func selectedMaterialDensity(materials []domain.MaterialDefinition, selected uint) float64 {
	if int(selected) >= len(materials) {
		return 1.24
	}
	return materials[selected].DensityGCM3
}

func (f *spoolForm) Widget() *adw.PreferencesGroup {
	return f.root
}

func (f *spoolForm) Spool(materials []domain.MaterialDefinition, base domain.Spool) (domain.Spool, bool) {
	idx := f.materialCombo.Selected()
	if int(idx) >= len(materials) {
		return domain.Spool{}, false
	}
	weight, _ := strconv.ParseFloat(f.weightEntry.Text(), 64)
	price, _ := strconv.ParseFloat(f.priceEntry.Text(), 64)
	diameter, _ := strconv.ParseFloat(strings.TrimSpace(f.diameterEntry.Text()), 64)
	density, _ := strconv.ParseFloat(strings.TrimSpace(f.densityEntry.Text()), 64)
	base.Manufacturer = f.manufacturerEntry.Text()
	base.Material = materials[idx].Name
	base.Color = f.colorEntry.Text()
	base.ColorHex = f.colorPicker.Hex()
	base.DiameterMM = diameter
	base.DensityGCM3 = density
	base.TotalWeightG = weight
	base.Price = price
	return base, true
}

func (f *spoolForm) Valid(materials []domain.MaterialDefinition) bool {
	if int(f.materialCombo.Selected()) >= len(materials) {
		return false
	}
	weight, err := strconv.ParseFloat(strings.TrimSpace(f.weightEntry.Text()), 64)
	if err != nil || weight <= 0 {
		return false
	}
	diameter, err := strconv.ParseFloat(strings.TrimSpace(f.diameterEntry.Text()), 64)
	if err != nil || diameter <= 0 {
		return false
	}
	density, err := strconv.ParseFloat(strings.TrimSpace(f.densityEntry.Text()), 64)
	if err != nil || density <= 0 {
		return false
	}
	priceText := strings.TrimSpace(f.priceEntry.Text())
	if priceText == "" {
		return true
	}
	price, err := strconv.ParseFloat(priceText, 64)
	return err == nil && price >= 0
}
