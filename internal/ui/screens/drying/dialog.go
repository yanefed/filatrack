package drying

import (
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func showAddDryingDialog(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onSave func()) {
	view := layout.Load("drying-dialog")

	t := i18n.L()
	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(t.AddDrying)

	form := newDryingForm(t, layout.Scope(view, "box_slot_1"))

	errorLabel := layout.Get[*gtk.Label](view, "errorLabel")

	notArchived := false
	spools, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &notArchived})
	if err != nil {
		errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
		errorLabel.SetVisible(true)
		spools = nil
	}
	form.SetSpools(t, spools, "")

	applyRecommendation := func() {
		profile, err := dryingProfileForSpool(ctx, dryingSvc, spools, form.spoolDropdown.Selected())
		if err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
		}
		form.SetRecommendation(profile)
	}
	form.spoolDropdown.NotifyProperty("selected", applyRecommendation)
	applyRecommendation()

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	cancelBtn.ConnectClicked(dialog.Destroy)

	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(t.AddDrying) })

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

	dialog.SetVisible(true)
}

func dryingProfileForSpool(ctx context.Context, dryingSvc *service.DryingService, spools []domain.Spool, selected uint) (*domain.DryingProfile, error) {
	if int(selected) >= len(spools) {
		return nil, nil
	}
	return dryingSvc.GetRecommendation(ctx, spools[selected].Material)
}
