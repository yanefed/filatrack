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
	var currency string
	var materials []domain.MaterialDefinition
	shared.RunAsyncUI(ctx, func() { currency = settings.GetCurrency(ctx); materials = availableMaterials(ctx, settings) }, func() { t.PricePH += " (" + currency + ")"; done(t, materials) })
}

func availableMaterials(ctx context.Context, settingsSvc *service.SettingsService) []domain.MaterialDefinition {
	materials, err := settingsSvc.ListMaterials(ctx)
	if err != nil {
		return nil
	}
	return materials
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
	cancelBtn.ConnectClicked(dialog.Destroy)

	actionLabel := t.Save
	if original == nil {
		actionLabel = t.AddSpoolBtn
	}
	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(actionLabel) })

	updateSaveState := func() {
		saveBtn.SetSensitive(form.Valid(materials))
	}
	form.weightEntry.NotifyProperty("text", updateSaveState)
	form.priceEntry.NotifyProperty("text", updateSaveState)
	form.diameterEntry.NotifyProperty("text", updateSaveState)
	form.densityEntry.NotifyProperty("text", updateSaveState)
	form.materialCombo.NotifyProperty("selected", updateSaveState)
	updateSaveState()
	saveAction := shared.Action(ctx, dialog)
	saveBtn.ConnectClicked(func() {
		base := domain.Spool{}
		if original != nil {
			base = *original
		}
		spool, ok := form.Spool(materials, base)
		if !ok {
			return
		}
		saveBtn.SetSensitive(false)
		saveAction(func() error { return save(spool) }, func(err error) {
			if err != nil {
				updateSaveState()
				errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				errorLabel.SetVisible(true)
				return
			}
			dialog.Destroy()
			if onSave != nil {
				onSave()
			}
		})
	})

	dialog.SetVisible(true)
	form.manufacturerEntry.GrabFocus()
}
