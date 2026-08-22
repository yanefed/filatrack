package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/widgets"
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type spoolForm struct {
	root              *gtk.Box
	manufacturerEntry *gtk.Entry
	materialCombo     *gtk.DropDown
	colorEntry        *gtk.Entry
	colorPicker       *widgets.ColorPicker
	weightEntry       *gtk.Entry
	priceEntry        *gtk.Entry
}

func newSpoolForm(t i18n.T, materials []domain.Material, spool *domain.Spool) *spoolForm {
	form := &spoolForm{root: gtk.NewBox(gtk.OrientationVertical, 6)}
	form.manufacturerEntry = gtk.NewEntry()
	form.manufacturerEntry.SetPlaceholderText(t.ManufacturerPH)
	form.root.Append(form.manufacturerEntry)

	names := make([]string, len(materials))
	selected := uint(0)
	for i, material := range materials {
		names[i] = string(material)
		if spool != nil && material == spool.Material {
			selected = uint(i)
		}
	}
	form.materialCombo = gtk.NewDropDown(gtk.NewStringList(names), nil)
	form.materialCombo.SetSelected(selected)
	form.root.Append(form.materialCombo)

	form.colorEntry = gtk.NewEntry()
	form.colorEntry.SetPlaceholderText(t.ColorPH)
	form.root.Append(form.colorEntry)
	colorHex := ""
	if spool != nil {
		colorHex = spool.ColorHex
	}
	form.colorPicker = widgets.NewColorPicker(t.ColorHexLabel, colorHex)
	form.root.Append(form.colorPicker.Widget())

	form.weightEntry = gtk.NewEntry()
	form.weightEntry.SetPlaceholderText(t.NetWeightPH)
	form.root.Append(form.weightEntry)
	form.priceEntry = gtk.NewEntry()
	form.priceEntry.SetPlaceholderText(t.PricePH)
	form.root.Append(form.priceEntry)

	if spool != nil {
		form.manufacturerEntry.SetText(spool.Manufacturer)
		form.colorEntry.SetText(spool.Color)
		form.weightEntry.SetText(fmt.Sprintf("%.0f", spool.TotalWeightG))
		form.priceEntry.SetText(fmt.Sprintf("%.2f", spool.Price))
	}
	return form
}

func (f *spoolForm) Widget() *gtk.Box {
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

func (f *spoolForm) Reset() {
	f.manufacturerEntry.SetText("")
	f.colorEntry.SetText("")
	f.colorPicker.SetHex("")
	f.weightEntry.SetText("")
	f.priceEntry.SetText("")
}
