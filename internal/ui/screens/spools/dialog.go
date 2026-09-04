package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func showAddSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onSave func()) {
	t := i18n.L()
	materials := availableMaterials(ctx, settingsSvc)
	showSpoolFormDialog(t, parent, t.AddSpoolBtn, materials, nil, func(spool domain.Spool) error {
		return spoolSvc.Create(ctx, &spool)
	}, onSave)
}

func showEditSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	t := i18n.L()
	materials := availableMaterials(ctx, settingsSvc)
	showSpoolFormDialog(t, parent, t.Edit, materials, &spool, func(updated domain.Spool) error {
		return spoolSvc.Update(ctx, &updated)
	}, onSave)
}

func availableMaterials(ctx context.Context, settingsSvc *service.SettingsService) []domain.Material {
	materials, err := settingsSvc.AllMaterials(ctx)
	if err != nil || len(materials) == 0 {
		return domain.BuiltInMaterials
	}
	return materials
}

func showSpoolFormDialog(t i18n.T, parent *gtk.Window, title string, materials []domain.Material, original *domain.Spool, save func(domain.Spool) error, onSave func()) {
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(title)
	dialog.SetDefaultSize(480, -1)

	box := widgets.NewDialogContent(16)
	form := newSpoolForm(t, materials, original)
	box.Append(form.Widget())

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

	actionLabel := t.Save
	if original == nil {
		actionLabel = t.AddSpoolBtn
	}
	saveBtn := gtk.NewButtonWithLabel(actionLabel)
	saveBtn.AddCSSClass("suggested-action")
	updateSaveState := func() {
		saveBtn.SetSensitive(form.Valid(materials))
	}
	form.weightEntry.NotifyProperty("text", updateSaveState)
	form.priceEntry.NotifyProperty("text", updateSaveState)
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
	actions.Append(saveBtn)
	box.Append(actions)

	dialog.SetChild(box)
	dialog.SetVisible(true)
	form.manufacturerEntry.GrabFocus()
}
