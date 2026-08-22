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

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSettingsScreen создаёт экран настроек приложения.
func NewSettingsScreen(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func(), onLanguageChanged func()) (*gtk.Box, func()) {
	box := widgets.NewScreen(i18n.L().Settings)

	content := gtk.NewBox(gtk.OrientationVertical, 18)
	content.SetMarginTop(12)
	box.Append(widgets.NewScrolled(content))

	content.Append(newLanguageSettingsSection(ctx, settingsSvc, onLanguageChanged))
	content.Append(newPrintSettingsSection(ctx, settingsSvc, onChanged))

	dryingSection, refreshProfiles := newDryingProfilesSection(ctx, parent, settingsSvc, onChanged)
	content.Append(dryingSection)

	matSection, refreshMaterials := newMaterialsSection(ctx, parent, settingsSvc, refreshProfiles, onChanged)
	content.Append(matSection)

	refreshSettings := func() {
		refreshProfiles()
		refreshMaterials()
	}

	return box, refreshSettings
}

// showEditProfileDialog opens a window for editing a drying profile.
func showEditProfileDialog(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, profile *domain.DryingProfile, onSave func()) {
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s — %s", i18n.L().Edit, string(profile.Material)))
	dialog.SetDefaultSize(350, 250)

	box := widgets.NewDialogContent(8)

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

	btnRow := widgets.NewActions(8, gtk.AlignFill)

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
		dialog.Destroy()
	})
	btnRow.Append(saveBtn)

	cancelBtn := gtk.NewButtonWithLabel(i18n.L().Close)
	cancelBtn.ConnectClicked(func() {
		dialog.Destroy()
	})
	btnRow.Append(cancelBtn)

	box.Append(btnRow)
	dialog.SetChild(box)
	dialog.SetVisible(true)
}

func showCustomMaterialDryingDialog(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, material, currentProfile domain.Material, onSave func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s — %s", t.DryingRule, material))
	dialog.SetDefaultSize(440, 360)

	box := widgets.NewDialogContent(10)

	profiles, err := settingsSvc.ListDryingProfiles(ctx)
	if err != nil {
		profiles = nil
	}
	profileNames := make([]string, len(profiles))
	selected := uint(0)
	for i, profile := range profiles {
		profileNames[i] = fmt.Sprintf("%s — %d°C, %.1f %s, %d %s",
			profile.Material, profile.TemperatureC, profile.DurationHours, t.Hours, profile.MaxDaysNoDry, t.Days)
		if profile.Material == currentProfile {
			selected = uint(i)
		}
	}
	box.Append(widgets.NewSectionTitle(t.UseDryingRule))
	profileDropdown := gtk.NewDropDown(gtk.NewStringList(profileNames), nil)
	profileDropdown.SetSelected(selected)
	profileDropdown.SetSensitive(len(profiles) > 0)
	box.Append(profileDropdown)
	bindBtn := gtk.NewButtonWithLabel(t.UseDryingRule)
	bindBtn.SetSensitive(len(profiles) > 0)
	box.Append(bindBtn)

	separator := gtk.NewSeparator(gtk.OrientationHorizontal)
	box.Append(separator)
	box.Append(widgets.NewSectionTitle(t.CreateDryingRule))

	tempEntry := gtk.NewEntry()
	tempEntry.SetPlaceholderText(t.TemperatureC)
	box.Append(tempEntry)
	durationEntry := gtk.NewEntry()
	durationEntry.SetPlaceholderText(t.DurationHours)
	box.Append(durationEntry)
	daysEntry := gtk.NewEntry()
	daysEntry.SetPlaceholderText(t.MaxDaysNoDry)
	box.Append(daysEntry)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetHAlign(gtk.AlignStart)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("dim-label")
	errorLabel.SetVisible(false)
	box.Append(errorLabel)

	finish := func(err error) {
		if err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
			return
		}
		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	}
	bindBtn.ConnectClicked(func() {
		idx := profileDropdown.Selected()
		if int(idx) >= len(profiles) {
			return
		}
		finish(settingsSvc.BindCustomMaterialDryingProfile(ctx, material, profiles[idx].Material))
	})

	actions := widgets.NewActions(8, gtk.AlignFill)
	createBtn := gtk.NewButtonWithLabel(t.CreateDryingRule)
	createBtn.ConnectClicked(func() {
		temperature, _ := strconv.Atoi(tempEntry.Text())
		duration, _ := strconv.ParseFloat(durationEntry.Text(), 64)
		days, _ := strconv.Atoi(daysEntry.Text())
		finish(settingsSvc.CreateCustomMaterialDryingProfile(ctx, material, &domain.DryingProfile{
			TemperatureC:  temperature,
			DurationHours: duration,
			MaxDaysNoDry:  days,
		}))
	})
	actions.Append(createBtn)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(dialog.Destroy)
	actions.Append(cancelBtn)
	box.Append(actions)

	dialog.SetChild(box)
	dialog.SetVisible(true)
}
