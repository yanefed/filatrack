package screens

import (
	"context"
	"fmt"
	"strconv"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSettingsScreen создаёт экран настроек приложения.
func NewSettingsScreen(ctx context.Context, settingsSvc *service.SettingsService, onChanged func(), onLanguageChanged func()) (*gtk.Box, func()) {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	title := gtk.NewLabel(i18n.L().SettingsTitle)
	title.AddCSSClass("title-1")
	box.Append(title)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)

	content := gtk.NewBox(gtk.OrientationVertical, 18)
	content.SetMarginTop(12)
	scrolled.SetChild(content)
	box.Append(scrolled)

	// === Language selection ===
	langSection := gtk.NewBox(gtk.OrientationVertical, 6)
	langLabel := gtk.NewLabel(i18n.L().LanguageLabel)
	langLabel.SetHAlign(gtk.AlignStart)
	langLabel.AddCSSClass("title-3")
	langSection.Append(langLabel)

	langs := i18n.SupportedLanguages()
	var langNames []string
	currentIdx := 0
	for idx, l := range langs {
		langNames = append(langNames, l.Name)
		if l.Code == i18n.GetLanguage() {
			currentIdx = idx
		}
	}
	langDropdown := gtk.NewDropDown(gtk.NewStringList(langNames), nil)
	langDropdown.SetSelected(uint(currentIdx))

	langSaveBtn := gtk.NewButtonWithLabel(i18n.L().Save)
	langSaveBtn.ConnectClicked(func() {
		idx := langDropdown.Selected()
		if int(idx) >= len(langs) {
			return
		}
		if err := settingsSvc.SetLanguage(ctx, langs[idx].Code); err != nil {
			return
		}
		if onLanguageChanged != nil {
			onLanguageChanged()
		}
	})

	langRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	langRow.Append(langDropdown)
	langRow.Append(langSaveBtn)
	langSection.Append(langRow)

	langNote := gtk.NewLabel(i18n.L().RestartNote)
	langNote.SetHAlign(gtk.AlignStart)
	langNote.AddCSSClass("dim-label")
	langSection.Append(langNote)

	content.Append(langSection)

	// === Drying profiles ===
	dryingSection := gtk.NewBox(gtk.OrientationVertical, 6)
	dryingLabel := gtk.NewLabel(i18n.L().DryingProfiles)
	dryingLabel.SetHAlign(gtk.AlignStart)
	dryingLabel.AddCSSClass("title-3")
	dryingSection.Append(dryingLabel)

	profilesList := gtk.NewListBox()
	profilesList.SetSelectionMode(gtk.SelectionNone)
	dryingSection.Append(profilesList)

	var refreshProfiles func()
	refreshProfiles = func() {
		for {
			row := profilesList.RowAtIndex(0)
			if row == nil {
				break
			}
			profilesList.Remove(row)
		}

		profiles, err := settingsSvc.ListDryingProfiles(ctx)
		if err != nil {
			return
		}

		for _, p := range profiles {
			profile := p
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(4)
			row.SetMarginBottom(4)

			info := gtk.NewLabel(fmt.Sprintf("%s: %d°C, %.0f %s, %d %s",
				string(profile.Material), profile.TemperatureC,
				profile.DurationHours, i18n.L().Hours,
				profile.MaxDaysNoDry, i18n.L().Days))
			info.SetHAlign(gtk.AlignStart)
			info.SetHExpand(true)
			row.Append(info)

			editBtn := gtk.NewButtonWithLabel(i18n.L().Edit)
			editBtn.ConnectClicked(func() {
				showEditProfileDialog(ctx, settingsSvc, &profile, func() {
					refreshProfiles()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			row.Append(editBtn)

			profilesList.Append(row)
		}
	}

	refreshProfiles()
	content.Append(dryingSection)

	// === Custom materials ===
	matSection := gtk.NewBox(gtk.OrientationVertical, 6)
	matLabel := gtk.NewLabel(i18n.L().CustomMaterials)
	matLabel.SetHAlign(gtk.AlignStart)
	matLabel.AddCSSClass("title-3")
	matSection.Append(matLabel)

	matList := gtk.NewListBox()
	matList.SetSelectionMode(gtk.SelectionNone)
	matSection.Append(matList)

	var refreshMaterials func()
	refreshMaterials = func() {
		for {
			row := matList.RowAtIndex(0)
			if row == nil {
				break
			}
			matList.Remove(row)
		}

		allMats, err := settingsSvc.AllMaterials(ctx)
		if err != nil {
			return
		}

		for _, m := range allMats {
			mat := m
			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(2)
			row.SetMarginBottom(2)

			label := gtk.NewLabel(string(mat))
			label.SetHAlign(gtk.AlignStart)
			label.SetHExpand(true)
			row.Append(label)

			if settingsSvc.IsBuiltIn(mat) {
				builtIn := gtk.NewLabel(i18n.L().BuiltInMaterial)
				builtIn.AddCSSClass("dim-label")
				row.Append(builtIn)
			} else {
				delBtn := gtk.NewButtonWithLabel(i18n.L().Delete)
				delBtn.ConnectClicked(func() {
					_ = settingsSvc.DeleteCustomMaterial(ctx, string(mat))
					refreshMaterials()
					if onChanged != nil {
						onChanged()
					}
				})
				row.Append(delBtn)
			}

			matList.Append(row)
		}
	}

	refreshMaterials()

	// Add material form
	addRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	addRow.SetMarginTop(6)
	matEntry := gtk.NewEntry()
	matEntry.SetPlaceholderText(i18n.L().MaterialName)
	matEntry.SetHExpand(true)
	addRow.Append(matEntry)

	addBtn := gtk.NewButtonWithLabel(i18n.L().AddMaterial)
	addBtn.ConnectClicked(func() {
		name := matEntry.Text()
		if name == "" {
			return
		}
		if err := settingsSvc.AddCustomMaterial(ctx, name); err != nil {
			return
		}
		matEntry.SetText("")
		refreshMaterials()
		if onChanged != nil {
			onChanged()
		}
	})
	addRow.Append(addBtn)
	matSection.Append(addRow)

	content.Append(matSection)

	refreshSettings := func() {
		refreshProfiles()
		refreshMaterials()
	}

	return box, refreshSettings
}

// showEditProfileDialog opens a window for editing a drying profile.
func showEditProfileDialog(ctx context.Context, settingsSvc *service.SettingsService, profile *domain.DryingProfile, onSave func()) {
	dialog := gtk.NewWindow()
	dialog.SetTitle(fmt.Sprintf("%s — %s", i18n.L().Edit, string(profile.Material)))
	dialog.SetDefaultSize(350, 250)
	dialog.SetModal(true)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)

	matLabel := gtk.NewLabel(string(profile.Material))
	matLabel.AddCSSClass("title-3")
	box.Append(matLabel)

	tempEntry := gtk.NewEntry()
	tempEntry.SetPlaceholderText(i18n.L().TemperatureC)
	tempEntry.SetText(strconv.Itoa(profile.TemperatureC))
	box.Append(tempEntry)

	durEntry := gtk.NewEntry()
	durEntry.SetPlaceholderText(i18n.L().DurationHours)
	durEntry.SetText(fmt.Sprintf("%.1f", profile.DurationHours))
	box.Append(durEntry)

	daysEntry := gtk.NewEntry()
	daysEntry.SetPlaceholderText(i18n.L().MaxDaysNoDry)
	daysEntry.SetText(strconv.Itoa(profile.MaxDaysNoDry))
	box.Append(daysEntry)

	btnRow := gtk.NewBox(gtk.OrientationHorizontal, 8)

	saveBtn := gtk.NewButtonWithLabel(i18n.L().Save)
	saveBtn.ConnectClicked(func() {
		temp, _ := strconv.Atoi(tempEntry.Text())
		dur, _ := strconv.ParseFloat(durEntry.Text(), 64)
		days, _ := strconv.Atoi(daysEntry.Text())

		updated := &domain.DryingProfile{
			Material:      profile.Material,
			TemperatureC:  temp,
			DurationHours: dur,
			MaxDaysNoDry:  days,
		}
		if err := settingsSvc.SaveDryingProfile(ctx, updated); err != nil {
			return
		}
		onSave()
		dialog.Close()
	})
	btnRow.Append(saveBtn)

	cancelBtn := gtk.NewButtonWithLabel(i18n.L().Close)
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	btnRow.Append(cancelBtn)

	box.Append(btnRow)
	dialog.SetChild(box)
	dialog.Show()
}
