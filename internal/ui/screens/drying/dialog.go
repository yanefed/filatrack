package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func showAddDryingDialog(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onSave func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(t.AddDrying)
	dialog.SetDefaultSize(480, -1)

	box := widgets.NewDialogContent(16)
	form := newDryingForm(t)
	box.Append(form.root)

	errorLabel := gtk.NewLabel("")
	errorLabel.AddCSSClass("error")
	errorLabel.SetHAlign(gtk.AlignStart)
	errorLabel.SetWrap(true)
	errorLabel.SetVisible(false)
	box.Append(errorLabel)

	notArchived := false
	spools, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
	if err != nil {
		errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
		errorLabel.SetVisible(true)
		spools = nil
	}
	form.SetSpools(t, spools, "")

	applyRecommendation := func() {
		form.SetRecommendation(dryingProfileForSpool(ctx, dryingSvc, spools, form.spoolDropdown.Selected()))
	}
	form.spoolDropdown.NotifyProperty("selected", applyRecommendation)
	applyRecommendation()

	actions := widgets.NewActions(8, gtk.AlignEnd)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(dialog.Destroy)
	actions.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel(t.AddDrying)
	saveBtn.AddCSSClass("suggested-action")
	updateSaveState := func() {
		saveBtn.SetSensitive(form.Valid(spools))
	}
	form.spoolDropdown.NotifyProperty("selected", updateSaveState)
	form.tempEntry.NotifyProperty("text", updateSaveState)
	form.durationEntry.NotifyProperty("text", updateSaveState)
	updateSaveState()
	saveBtn.ConnectClicked(func() {
		session, ok := form.Session(spools)
		if !ok {
			return
		}
		if err := dryingSvc.Create(ctx, session); err != nil {
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
}

func dryingProfileForSpool(ctx context.Context, dryingSvc *service.DryingService, spools []domain.Spool, selected uint) *domain.DryingProfile {
	if int(selected) >= len(spools) {
		return nil
	}
	return dryingSvc.GetRecommendation(ctx, spools[selected].Material)
}
