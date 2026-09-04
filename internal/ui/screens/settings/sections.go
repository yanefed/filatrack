package settings

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newLanguageSettingsSection(ctx context.Context, settingsSvc *service.SettingsService, onLanguageChanged func()) *gtk.Box {
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(i18n.L().LanguageLabel))
	languages := i18n.SupportedLanguages()
	names := make([]string, len(languages))
	selected := uint(0)
	for i, language := range languages {
		names[i] = language.Name
		if language.Code == i18n.GetLanguage() {
			selected = uint(i)
		}
	}
	dropdown := gtk.NewDropDown(gtk.NewStringList(names), nil)
	dropdown.SetSelected(selected)
	saveBtn := gtk.NewButtonWithLabel(i18n.L().Save)
	saveBtn.ConnectClicked(func() {
		idx := dropdown.Selected()
		if int(idx) >= len(languages) || settingsSvc.SetLanguage(ctx, languages[idx].Code) != nil {
			return
		}
		if onLanguageChanged != nil {
			onLanguageChanged()
		}
	})
	row := gtk.NewBox(gtk.OrientationHorizontal, 6)
	row.Append(dropdown)
	row.Append(saveBtn)
	section.Append(row)
	return section
}

func newPrintSettingsSection(ctx context.Context, settingsSvc *service.SettingsService, onChanged func()) *gtk.Box {
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(i18n.L().Print))
	allowLowFilament := gtk.NewCheckButtonWithLabel(i18n.L().LowFilamentSetting)
	allowLowFilament.SetActive(settingsSvc.AllowLowFilamentWarning(ctx))
	allowLowFilament.ConnectToggled(func() {
		_ = settingsSvc.SetAllowLowFilamentWarning(ctx, allowLowFilament.Active())
		if onChanged != nil {
			onChanged()
		}
	})
	section.Append(allowLowFilament)

	row := gtk.NewBox(gtk.OrientationHorizontal, 6)
	label := gtk.NewLabel(i18n.L().LowFilamentLevel)
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	row.Append(label)
	level := gtk.NewSpinButtonWithRange(0, 1_000_000, 1)
	level.SetValue(float64(settingsSvc.GetLowFilamentLevel(ctx)))
	row.Append(level)
	saveBtn := gtk.NewButtonWithLabel(i18n.L().Save)
	saveBtn.ConnectClicked(func() {
		if settingsSvc.SetLowFilamentLevel(ctx, level.ValueAsInt()) != nil {
			return
		}
		if onChanged != nil {
			onChanged()
		}
	})
	row.Append(saveBtn)
	section.Append(row)
	return section
}

func newMaterialsSection(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(i18n.L().MaterialSettings))
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)
	section.Append(list)

	var refresh func()
	refresh = func() {
		widgets.ClearListBox(list)
		materials, err := settingsSvc.ListMaterials(ctx)
		if err != nil {
			return
		}
		for _, item := range materials {
			material := item
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(2)
			row.SetMarginBottom(2)
			label := gtk.NewLabel(fmt.Sprintf("%s · %.3g g/cm³ · %d°C · %g %s · %d %s",
				material.Name, material.DensityGCM3, material.DryingTemperatureC,
				material.DryingDurationHours, i18n.L().Hours, material.MaxDaysNoDry, i18n.L().Days))
			label.SetHAlign(gtk.AlignStart)
			label.SetHExpand(true)
			row.Append(label)
			editBtn := gtk.NewButtonWithLabel(i18n.L().Edit)
			editBtn.ConnectClicked(func() {
				showMaterialDialog(ctx, parent, settingsSvc, &material, refresh, onChanged)
			})
			row.Append(editBtn)
			deleteBtn := gtk.NewButtonWithLabel(i18n.L().Delete)
			deleteBtn.ConnectClicked(func() {
				_ = settingsSvc.DeleteMaterial(ctx, material.Name)
				refresh()
				if onChanged != nil {
					onChanged()
				}
			})
			row.Append(deleteBtn)
			list.Append(row)
		}
	}

	addBtn := gtk.NewButtonWithLabel(i18n.L().AddMaterial)
	addBtn.ConnectClicked(func() {
		showMaterialDialog(ctx, parent, settingsSvc, nil, refresh, onChanged)
	})
	addBtn.SetHAlign(gtk.AlignStart)
	section.Append(addBtn)
	refresh()
	return section, refresh
}

func showMaterialDialog(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, original *domain.MaterialDefinition, refresh, onChanged func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	title := t.AddMaterial
	if original != nil {
		title = fmt.Sprintf("%s — %s", t.Edit, original.Name)
	}
	dialog.SetTitle(title)
	dialog.SetDefaultSize(480, -1)
	box := widgets.NewDialogContent(16)

	materialGroup := adw.NewPreferencesGroup()
	materialGroup.SetTitle(t.MaterialSettings)
	nameEntry := newMaterialEntry(t.MaterialName, false)
	densityEntry := newMaterialEntry(t.FilamentDensity, true)
	materialGroup.Add(nameEntry)
	materialGroup.Add(densityEntry)
	box.Append(materialGroup)

	dryingGroup := adw.NewPreferencesGroup()
	dryingGroup.SetTitle(t.Drying)
	temperatureEntry := newMaterialEntry(t.TemperatureC, true)
	durationEntry := newMaterialEntry(t.DurationHours, true)
	daysEntry := newMaterialEntry(t.MaxDaysNoDry, true)
	dryingGroup.Add(temperatureEntry)
	dryingGroup.Add(durationEntry)
	dryingGroup.Add(daysEntry)
	box.Append(dryingGroup)

	if original == nil {
		densityEntry.SetText("1.24")
		temperatureEntry.SetText("45")
		durationEntry.SetText("4")
		daysEntry.SetText("30")
	} else {
		nameEntry.SetText(string(original.Name))
		nameEntry.SetSensitive(false)
		densityEntry.SetText(strconv.FormatFloat(original.DensityGCM3, 'f', -1, 64))
		temperatureEntry.SetText(strconv.Itoa(original.DryingTemperatureC))
		durationEntry.SetText(strconv.FormatFloat(original.DryingDurationHours, 'f', -1, 64))
		daysEntry.SetText(strconv.Itoa(original.MaxDaysNoDry))
	}
	errorLabel := gtk.NewLabel("")
	errorLabel.AddCSSClass("error")
	errorLabel.SetHAlign(gtk.AlignStart)
	errorLabel.SetWrap(true)
	errorLabel.SetVisible(false)
	box.Append(errorLabel)

	actions := widgets.NewActions(8, gtk.AlignEnd)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(dialog.Destroy)
	actions.Append(cancelBtn)
	saveBtn := gtk.NewButtonWithLabel(t.Save)
	saveBtn.AddCSSClass("suggested-action")
	materialFromForm := func() *domain.MaterialDefinition {
		density, _ := strconv.ParseFloat(strings.TrimSpace(densityEntry.Text()), 64)
		temperature, _ := strconv.Atoi(strings.TrimSpace(temperatureEntry.Text()))
		duration, _ := strconv.ParseFloat(strings.TrimSpace(durationEntry.Text()), 64)
		days, _ := strconv.Atoi(strings.TrimSpace(daysEntry.Text()))
		return &domain.MaterialDefinition{
			Name: domain.Material(strings.TrimSpace(nameEntry.Text())), DensityGCM3: density,
			DryingTemperatureC: temperature, DryingDurationHours: duration, MaxDaysNoDry: days,
		}
	}
	updateSaveState := func() {
		saveBtn.SetSensitive(materialFromForm().Validate() == nil)
		errorLabel.SetVisible(false)
	}
	for _, entry := range []*adw.EntryRow{nameEntry, densityEntry, temperatureEntry, durationEntry, daysEntry} {
		entry.NotifyProperty("text", updateSaveState)
	}
	updateSaveState()
	saveBtn.ConnectClicked(func() {
		material := materialFromForm()
		var err error
		if original == nil {
			err = settingsSvc.CreateMaterial(ctx, material)
		} else {
			err = settingsSvc.UpdateMaterial(ctx, material)
		}
		if err != nil {
			errorLabel.SetText(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
			return
		}
		dialog.Destroy()
		refresh()
		if onChanged != nil {
			onChanged()
		}
	})
	actions.Append(saveBtn)
	box.Append(actions)
	dialog.SetChild(box)
	dialog.SetVisible(true)
	if original == nil {
		nameEntry.GrabFocus()
	} else {
		densityEntry.GrabFocus()
	}
}

func newMaterialEntry(title string, numeric bool) *adw.EntryRow {
	entry := adw.NewEntryRow()
	entry.SetTitle(title)
	if numeric {
		entry.SetInputPurpose(gtk.InputPurposeNumber)
	}
	return entry
}
