package spools

import (
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
)

func showAddSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onSave func()) {
	prepareSpoolDialog(ctx, parent, settingsSvc, func(t i18n.T, materials []domain.MaterialDefinition) {
		showSpoolFormDialog(ctx, t, parent, t.AddSpoolBtn, materials, nil, func(spool domain.Spool) error { return spoolSvc.Create(ctx, &spool) }, onSave)
	})
}

func showEditSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	prepareSpoolDialog(ctx, parent, settingsSvc, func(t i18n.T, materials []domain.MaterialDefinition) {
		showSpoolFormDialog(ctx, t, parent, t.Edit, materials, &spool, func(updated domain.Spool) error { return spoolSvc.Update(ctx, &updated) }, onSave)
	})
}

func prepareSpoolDialog(ctx context.Context, parent *gtk.Window, settings *service.SettingsService, done func(i18n.T, []domain.MaterialDefinition)) {

	t := i18n.L()
	view := layout.Load("modal-window")
	dialog := layout.Get[*gtk.Window](view, "dialog")
	dialog.SetTitle(t.Spools)
	if parent != nil {
		dialog.SetTransientFor(parent)
		dialog.SetDestroyWithParent(true)
	}
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)
	dialog.SetChild(box)
	status := shared.InlineMessage(box)
	cancel := gtk.NewButtonWithLabel(t.Cancel)
	box.Append(cancel)
	cancel.ConnectClicked(dialog.Destroy)
	type initial struct {
		currency  string
		materials []domain.MaterialDefinition
		err       error
	}
	loader := shared.NewLoader[initial](ctx, dialog)
	var load func()
	feedback := shared.NewReadFeedback(box, status, func() { load() })
	load = func() {
		feedback.Begin()
		loader.Request(func(c context.Context) initial {
			materials, err := settings.ListMaterials(c)
			return initial{settings.GetCurrency(c), materials, err}
		}, func(data initial) {
			if data.err != nil {
				feedback.Failure(data.err)
				return
			}
			dialog.Destroy()
			t.PricePH += " (" + data.currency + ")"
			done(t, data.materials)
		})
	}
	load()
	dialog.Present()
}

func showSpoolFormDialog(ctx context.Context, t i18n.T, parent *gtk.Window, title string, materials []domain.MaterialDefinition, original *domain.Spool, save func(domain.Spool) error, onSave func()) {
	view := layout.Load("spool-dialog")

	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(title)

	form := newSpoolForm(t, materials, original, layout.Scope(view, "box_slot_1"))

	errorLabel := layout.Get[*gtk.Label](view, "errorLabel")

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	busy, attempted := false, false
	closeForm, _ := shared.GuardChanges(dialog, func() string { return shared.FormSnapshot(form.root) + form.colorPicker.Hex() }, func() bool { return busy })
	cancelBtn.ConnectClicked(closeForm)

	actionLabel := t.Save
	if original == nil {
		actionLabel = t.AddSpoolBtn
	}
	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(actionLabel) })

	updateSaveState := func() {
		saveBtn.SetSensitive(!busy)
		if attempted {
			form.Validate(t, materials, false)
		}
	}
	form.weightEntry.NotifyProperty("text", updateSaveState)
	form.priceEntry.NotifyProperty("text", updateSaveState)
	form.diameterEntry.NotifyProperty("text", updateSaveState)
	form.densityEntry.NotifyProperty("text", updateSaveState)
	form.materialCombo.NotifyProperty("selected", updateSaveState)
	updateSaveState()
	saveAction := shared.Action(ctx, dialog)
	saveBtn.ConnectClicked(func() {
		attempted = true
		if !form.Validate(t, materials, true) {
			return
		}
		base := domain.Spool{}
		if original != nil {
			base = *original
		}
		spool, ok := form.Spool(materials, base)
		if !ok {
			return
		}
		busy = true
		form.root.SetSensitive(false)
		cancelBtn.SetSensitive(false)
		saveBtn.SetLabel(t.Saving)
		saveBtn.SetSensitive(false)
		saveAction(func() error { return save(spool) }, func(err error) {
			busy = false
			if err != nil {
				form.root.SetSensitive(true)
				cancelBtn.SetSensitive(true)
				saveBtn.SetLabel(actionLabel)
				updateSaveState()
				errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				errorLabel.SetVisible(true)
				return
			}
			shared.Notify(parent, t.Saved, "", nil)
			dialog.Destroy()
			if onSave != nil {
				onSave()
			}
		})
	})

	dialog.SetVisible(true)
	form.manufacturerEntry.GrabFocus()
}
