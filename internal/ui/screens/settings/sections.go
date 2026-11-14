package settings

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

func newLanguageSettingsSection(ctx context.Context, settingsSvc *service.SettingsService, onLanguageChanged func(), views ...*layout.View) *gtk.Box {
	view := layout.Use("language-settings", views...)

	section := layout.Get[*gtk.Box](view, "section")
	_ = layout.Configure[*gtk.Label](view, "object_1232", func(w *gtk.Label) { w.SetLabel(i18n.L().LanguageLabel) })
	languages := i18n.SupportedLanguages()
	names := make([]string, len(languages))
	selected := uint(0)
	for i, language := range languages {
		names[i] = language.Name
		if language.Code == i18n.GetLanguage() {
			selected = uint(i)
		}
	}
	dropdown := layout.Configure[*gtk.DropDown](view, "dropdown", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(names)) })
	dropdown.SetSelected(selected)
	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(i18n.L().Save) })
	action := shared.Action(ctx, section)
	saveBtn.ConnectClicked(func() {
		idx := dropdown.Selected()
		if int(idx) >= len(languages) {
			return
		}
		lang := languages[idx].Code
		saveBtn.SetSensitive(false)
		action(func() error { return settingsSvc.SetLanguage(ctx, lang) }, func(err error) {
			saveBtn.SetSensitive(true)
			if err == nil && onLanguageChanged != nil {
				onLanguageChanged()
			}
		})
	})

	return section
}

func newPrintSettingsSection(ctx context.Context, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) *gtk.Box {
	view := layout.Use("print-settings", views...)

	section := layout.Get[*gtk.Box](view, "section")
	_ = layout.Configure[*gtk.Label](view, "object_1245", func(w *gtk.Label) { w.SetLabel(i18n.L().Print) })
	allowLowFilament := layout.Configure[*gtk.CheckButton](view, "allowLowFilament", func(w *gtk.CheckButton) { w.SetLabel(i18n.L().LowFilamentSetting) })
	updating := false
	action := shared.Action(ctx, section)
	allowLowFilament.ConnectToggled(func() {
		if updating {
			return
		}
		value := allowLowFilament.Active()
		allowLowFilament.SetSensitive(false)
		action(func() error { return settingsSvc.SetAllowLowFilamentWarning(ctx, value) }, func(err error) {
			allowLowFilament.SetSensitive(true)
			if err == nil && onChanged != nil {
				onChanged()
			}
		})
	})
	_ = layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(i18n.L().LowFilamentLevel) })

	level := layout.Get[*gtk.SpinButton](view, "level")
	type values struct {
		allow bool
		level int
	}
	section.SetSensitive(false)
	loader := shared.NewLoader[values](ctx, section)
	loader.Request(func(ctx context.Context) values {
		return values{settingsSvc.AllowLowFilamentWarning(ctx), settingsSvc.GetLowFilamentLevel(ctx)}
	}, func(v values) {
		section.SetSensitive(true)
		updating = true
		allowLowFilament.SetActive(v.allow)
		level.SetValue(float64(v.level))
		updating = false
	})

	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(i18n.L().Save) })
	levelAction := shared.Action(ctx, section)
	saveBtn.ConnectClicked(func() {
		value := level.ValueAsInt()
		saveBtn.SetSensitive(false)
		levelAction(func() error { return settingsSvc.SetLowFilamentLevel(ctx, value) }, func(err error) {
			saveBtn.SetSensitive(true)
			if err == nil && onChanged != nil {
				onChanged()
			}
		})
	})

	return section
}

func newMaterialsSection(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func()) {
	view := layout.Use("material-settings", views...)

	section := layout.Get[*gtk.Box](view, "section")
	_ = layout.Configure[*gtk.Label](view, "object_1266", func(w *gtk.Label) { w.SetLabel(i18n.L().MaterialSettings) })
	list := layout.Get[*gtk.ListBox](view, "list")

	type result struct {
		materials []domain.MaterialDefinition
		err       error
	}
	loader := shared.NewLoader[result](ctx, section)
	var previous []domain.MaterialDefinition
	var refresh func()
	refresh = func() {
		loader.Request(func(ctx context.Context) result { m, e := settingsSvc.ListMaterials(ctx); return result{m, e} }, func(data result) {
			if data.err != nil || reflect.DeepEqual(previous, data.materials) {
				return
			}
			previous = data.materials
			materials := data.materials
			widgets.ClearListBox(list)
			for _, item := range materials {
				view := layout.Load("material-row")

				material := item
				row := layout.Get[*gtk.Box](view, "row")
				_ = layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) {
					w.SetLabel(fmt.Sprintf("%s · %.3g g/cm³ · %d°C · %g %s · %d %s",
						material.Name, material.DensityGCM3, material.DryingTemperatureC,
						material.DryingDurationHours, i18n.L().Hours, material.MaxDaysNoDry, i18n.L().Days))
				})

				editBtn := layout.Configure[*gtk.Button](view, "editBtn", func(w *gtk.Button) { w.SetLabel(i18n.L().Edit) })
				editBtn.ConnectClicked(func() {
					showMaterialDialog(ctx, parent, settingsSvc, &material, refresh, onChanged)
				})

				deleteBtn := layout.Configure[*gtk.Button](view, "deleteBtn", func(w *gtk.Button) { w.SetLabel(i18n.L().Delete) })
				deleteAction := shared.Action(ctx, row)
				deleteBtn.ConnectClicked(func() {
					deleteBtn.SetSensitive(false)
					deleteAction(func() error { return settingsSvc.DeleteMaterial(ctx, material.Name) }, func(err error) {
						deleteBtn.SetSensitive(true)
						if err != nil {
							return
						}
						refresh()
						if onChanged != nil {
							onChanged()
						}
					})
				})

				list.Append(row)
			}
		})
	}

	addBtn := layout.Configure[*gtk.Button](view, "addBtn", func(w *gtk.Button) { w.SetLabel(i18n.L().AddMaterial) })
	addBtn.ConnectClicked(func() {
		showMaterialDialog(ctx, parent, settingsSvc, nil, refresh, onChanged)
	})

	refresh()
	return section, refresh
}

func showMaterialDialog(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, original *domain.MaterialDefinition, refresh, onChanged func()) {
	view := layout.Load("material-dialog")

	t := i18n.L()
	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	title := t.AddMaterial
	if original != nil {
		title = fmt.Sprintf("%s — %s", t.Edit, original.Name)
	}
	dialog.SetTitle(title)

	materialGroup := layout.Get[*adw.PreferencesGroup](view, "materialGroup")
	materialGroup.SetTitle(t.MaterialSettings)
	nameEntry := newMaterialEntry(t.MaterialName, false, layout.Scope(view, "materialGroup_slot_1"))
	densityEntry := newMaterialEntry(t.FilamentDensity, true, layout.Scope(view, "materialGroup_slot_2"))

	dryingGroup := layout.Get[*adw.PreferencesGroup](view, "dryingGroup")
	dryingGroup.SetTitle(t.Drying)
	temperatureEntry := newMaterialEntry(t.TemperatureC, true, layout.Scope(view, "dryingGroup_slot_1"))
	durationEntry := newMaterialEntry(t.DurationHours, true, layout.Scope(view, "dryingGroup_slot_2"))
	daysEntry := newMaterialEntry(t.MaxDaysNoDry, true, layout.Scope(view, "dryingGroup_slot_3"))

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
	errorLabel := layout.Get[*gtk.Label](view, "errorLabel")

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	cancelBtn.ConnectClicked(dialog.Destroy)

	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(t.Save) })

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
	action := shared.Action(ctx, dialog)
	saveBtn.ConnectClicked(func() {
		material := materialFromForm()
		saveBtn.SetSensitive(false)
		action(func() error {
			if original == nil {
				return settingsSvc.CreateMaterial(ctx, material)
			}
			return settingsSvc.UpdateMaterial(ctx, material)
		}, func(err error) {
			updateSaveState()
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
	})

	dialog.SetVisible(true)
	if original == nil {
		nameEntry.GrabFocus()
	} else {
		densityEntry.GrabFocus()
	}
}

func newMaterialEntry(title string, numeric bool, views ...*layout.View) *adw.EntryRow {
	view := layout.Use("material-entry", views...)

	entry := layout.Get[*adw.EntryRow](view, "entry")
	entry.SetTitle(title)
	if numeric {
		entry.SetInputPurpose(gtk.InputPurposeNumber)
	}
	return entry
}

func newCurrencySettingsSection(ctx context.Context, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) *gtk.Box {
	view := layout.Use("currency-settings", views...)

	t := i18n.L()
	section := layout.Get[*gtk.Box](view, "section")
	_ = layout.Configure[*gtk.Label](view, "object_1355", func(w *gtk.Label) { w.SetLabel(t.CurrencyLabel) })

	currency := layout.Configure[*gtk.DropDown](view, "currency", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(domain.Currencies)) })
	section.SetSensitive(false)
	loader := shared.NewLoader[string](ctx, section)
	loader.Request(settingsSvc.GetCurrency, func(selected string) {
		section.SetSensitive(true)
		for i, code := range domain.Currencies {
			if code == selected {
				currency.SetSelected(uint(i))
				break
			}
		}
	})

	message := layout.Get[*gtk.Label](view, "message")
	save := layout.Configure[*gtk.Button](view, "save", func(w *gtk.Button) { w.SetLabel(t.Save) })
	action := shared.Action(ctx, section)
	save.ConnectClicked(func() {
		idx := currency.Selected()
		if int(idx) >= len(domain.Currencies) {
			return
		}
		value := domain.Currencies[idx]
		save.SetSensitive(false)
		action(func() error { return settingsSvc.SetCurrency(ctx, value) }, func(err error) {
			save.SetSensitive(true)
			if err != nil {
				message.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
				message.SetVisible(true)
				return
			}
			message.SetVisible(false)
			if onChanged != nil {
				onChanged()
			}
		})
	})
	_ = layout.Configure[*gtk.Label](view, "help", func(w *gtk.Label) { w.SetLabel(t.CurrencyHelp) })

	return section
}
