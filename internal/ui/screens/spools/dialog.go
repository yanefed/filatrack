package spools

import (
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func showAddSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onSave func()) {
	t := i18n.L()
	t.PricePH += " (" + settingsSvc.GetCurrency(ctx) + ")"
	materials := availableMaterials(ctx, settingsSvc)
	showSpoolFormDialog(t, parent, t.AddSpoolBtn, materials, nil, func(spool domain.Spool) error {
		return spoolSvc.Create(ctx, &spool)
	}, onSave)
}

func showEditSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	t := i18n.L()
	t.PricePH += " (" + settingsSvc.GetCurrency(ctx) + ")"
	materials := availableMaterials(ctx, settingsSvc)
	showSpoolFormDialog(t, parent, t.Edit, materials, &spool, func(updated domain.Spool) error {
		return spoolSvc.Update(ctx, &updated)
	}, onSave)
}

func availableMaterials(ctx context.Context, settingsSvc *service.SettingsService) []domain.MaterialDefinition {
	materials, err := settingsSvc.ListMaterials(ctx)
	if err != nil {
		return nil
	}
	return materials
}

func showSpoolFormDialog(t i18n.T, parent *gtk.Window, title string, materials []domain.MaterialDefinition, original *domain.Spool, save func(domain.Spool) error, onSave func()) {
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
	saveBtn.ConnectClicked(func() {
		base := domain.Spool{}
		if original != nil {
			base = *original
		}
		spool, ok := form.Spool(materials, base)
		if !ok {
			return
		}
		if err := save(spool); err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
			return
		}
		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	})

	dialog.SetVisible(true)
	form.manufacturerEntry.GrabFocus()
}
