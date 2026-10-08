package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// spoolFilters owns presentation state only; querying stays in SpoolService.
type spoolFilters struct {
	root          *gtk.Box
	manufacturer  *adw.ComboRow
	material      *adw.ComboRow
	reset         *gtk.Button
	manufacturers []string
	materials     []string
	updating      bool
	t             i18n.T
}

func newSpoolFilters(t i18n.T, onChanged func()) *spoolFilters {
	f := &spoolFilters{root: gtk.NewBox(gtk.OrientationHorizontal, 12), t: t}
	add := func(title, all string) *adw.ComboRow {
		group := adw.NewPreferencesGroup()
		group.SetHExpand(true)
		dropdown := adw.NewComboRow()
		dropdown.SetTitle(title)
		dropdown.SetUseSubtitle(true)
		dropdown.SetModel(gtk.NewStringList([]string{all}))
		dropdown.SetTooltipText(title)
		dropdown.SetEnableSearch(true)
		group.Add(dropdown)
		f.root.Append(group)
		return dropdown
	}
	f.manufacturer = add(t.ManufacturerPH, t.AllManufacturers)
	f.material = add(t.MaterialLabel, t.AllMaterials)
	f.reset = gtk.NewButtonFromIconName("edit-clear-symbolic")
	f.reset.SetTooltipText(t.ResetFilters)
	f.reset.AddCSSClass("flat")
	f.reset.AddCSSClass("circular")
	f.reset.SetVAlign(gtk.AlignCenter)
	f.reset.SetSensitive(false)
	f.root.Append(f.reset)
	changed := func() {
		if f.updating {
			return
		}
		f.updateReset()
		onChanged()
	}
	f.manufacturer.NotifyProperty("selected", changed)
	f.material.NotifyProperty("selected", changed)
	f.reset.ConnectClicked(func() {
		f.updating = true
		f.manufacturer.SetSelected(0)
		f.material.SetSelected(0)
		f.updating = false
		changed()
	})
	return f
}

func selectedFilterValue(dropdown *adw.ComboRow, values []string) *string {
	index := dropdown.Selected()
	if index == 0 || index == gtk.InvalidListPosition || int(index) > len(values) {
		return nil
	}
	value := values[index-1]
	return &value
}

func (f *spoolFilters) Filter() domain.SpoolFilter {
	archived := false
	filter := domain.SpoolFilter{Archived: &archived, Manufacturer: selectedFilterValue(f.manufacturer, f.manufacturers)}
	if material := selectedFilterValue(f.material, f.materials); material != nil {
		value := domain.Material(*material)
		filter.Material = &value
	}
	return filter
}

func (f *spoolFilters) SetOptions(options service.SpoolFilterOptions) {
	f.updating = true
	defer func() { f.updating = false }()
	set := func(dropdown *adw.ComboRow, previous, values []string, all string) {
		selected := selectedFilterValue(dropdown, previous)
		labels := []string{all}
		index := uint(0)
		for i, value := range values {
			label := value
			if label == "" {
				label = f.t.FilterUnspecified
			}
			labels = append(labels, label)
			if selected != nil && value == *selected {
				index = uint(i + 1)
			}
		}
		dropdown.SetModel(gtk.NewStringList(labels))
		dropdown.SetSelected(index)
	}
	materials := make([]string, len(options.Materials))
	for i, material := range options.Materials {
		materials[i] = string(material)
	}
	set(f.manufacturer, f.manufacturers, options.Manufacturers, f.t.AllManufacturers)
	set(f.material, f.materials, materials, f.t.AllMaterials)
	f.manufacturers = options.Manufacturers
	f.materials = materials
	f.updateReset()
}

func (f *spoolFilters) updateReset() {
	filter := f.Filter()
	f.reset.SetSensitive(filter.Manufacturer != nil || filter.Material != nil)
}
