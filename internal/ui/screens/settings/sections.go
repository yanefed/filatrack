package settings

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strings"

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
	section.Append(widgets.NewSectionTitle(i18n.L().Prints))
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

func newDryingProfilesSection(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(i18n.L().DryingProfiles))
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)
	section.Append(list)

	var refresh func()
	refresh = func() {
		widgets.ClearListBox(list)
		profiles, err := settingsSvc.ListDryingProfiles(ctx)
		if err != nil {
			return
		}
		for _, item := range profiles {
			profile := item
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(4)
			row.SetMarginBottom(4)
			info := gtk.NewLabel(fmt.Sprintf("%s: %d°C, %.0f %s, %d %s", profile.Material, profile.TemperatureC, profile.DurationHours, i18n.L().Hours, profile.MaxDaysNoDry, i18n.L().Days))
			info.SetHAlign(gtk.AlignStart)
			info.SetHExpand(true)
			row.Append(info)
			editBtn := gtk.NewButtonWithLabel(i18n.L().Edit)
			editBtn.ConnectClicked(func() {
				showEditProfileDialog(ctx, parent, settingsSvc, &profile, func() {
					refresh()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			row.Append(editBtn)
			list.Append(row)
		}
	}
	refresh()
	return section, refresh
}

func newMaterialsSection(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, refreshProfiles func(), onChanged func()) (*gtk.Box, func()) {
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(i18n.L().CustomMaterials))
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)
	section.Append(list)

	var refresh func()
	refresh = func() {
		widgets.ClearListBox(list)
		materials, err := settingsSvc.AllMaterials(ctx)
		if err != nil {
			return
		}
		bindings, err := settingsSvc.ListCustomMaterialBindings(ctx)
		if err != nil {
			return
		}
		profileByMaterial := make(map[domain.Material]domain.Material, len(bindings))
		for _, binding := range bindings {
			profileByMaterial[binding.Name] = binding.DryingProfileMaterial
		}
		for _, item := range materials {
			material := item
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(2)
			row.SetMarginBottom(2)
			label := gtk.NewLabel(string(material))
			label.SetHAlign(gtk.AlignStart)
			label.SetHExpand(true)
			row.Append(label)
			if settingsSvc.IsBuiltIn(material) {
				builtIn := gtk.NewLabel(i18n.L().BuiltInMaterial)
				builtIn.AddCSSClass("dim-label")
				row.Append(builtIn)
			} else {
				profileMaterial := profileByMaterial[material]
				profileText := i18n.L().NoDryingRule
				if profileMaterial != "" {
					profileText = fmt.Sprintf("%s: %s", i18n.L().DryingRule, profileMaterial)
				}
				profileLabel := gtk.NewLabel(profileText)
				profileLabel.AddCSSClass("dim-label")
				row.Append(profileLabel)
				ruleBtn := gtk.NewButtonWithLabel(i18n.L().DryingRule)
				ruleBtn.ConnectClicked(func() {
					showCustomMaterialDryingDialog(ctx, parent, settingsSvc, material, profileMaterial, func() {
						refreshProfiles()
						refresh()
						if onChanged != nil {
							onChanged()
						}
					})
				})
				row.Append(ruleBtn)
				deleteBtn := gtk.NewButtonWithLabel(i18n.L().Delete)
				deleteBtn.ConnectClicked(func() {
					_ = settingsSvc.DeleteCustomMaterial(ctx, string(material))
					refresh()
					if onChanged != nil {
						onChanged()
					}
				})
				row.Append(deleteBtn)
			}
			list.Append(row)
		}
	}

	addRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	addRow.SetMarginTop(6)
	entry := gtk.NewEntry()
	entry.SetPlaceholderText(i18n.L().MaterialName)
	entry.SetHExpand(true)
	addRow.Append(entry)
	addBtn := gtk.NewButtonWithLabel(i18n.L().AddMaterial)
	addBtn.ConnectClicked(func() {
		name := strings.TrimSpace(entry.Text())
		if name == "" || settingsSvc.AddCustomMaterial(ctx, name) != nil {
			return
		}
		entry.SetText("")
		refresh()
		if onChanged != nil {
			onChanged()
		}
		showCustomMaterialDryingDialog(ctx, parent, settingsSvc, domain.Material(name), "", func() {
			refreshProfiles()
			refresh()
			if onChanged != nil {
				onChanged()
			}
		})
	})
	addRow.Append(addBtn)
	section.Append(addRow)
	refresh()
	return section, refresh
}
