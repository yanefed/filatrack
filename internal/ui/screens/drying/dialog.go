package drying

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

	var spools []domain.Spool
	profiles := make(map[domain.Material]domain.DryingProfile)
	applyRecommendation := func() {
		var profile *domain.DryingProfile
		i := form.spoolDropdown.Selected()
		if int(i) < len(spools) {
			if p, ok := profiles[spools[i].Material]; ok {
				profile = &p
			}
		}
		form.SetRecommendation(profile)
	}
	form.spoolDropdown.NotifyProperty("selected", applyRecommendation)

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	busy, loaded, attempted := false, false, false
	closeForm, resetForm := shared.GuardChanges(dialog, func() string { return shared.FormSnapshot(form.root) }, func() bool { return busy })
	cancelBtn.ConnectClicked(closeForm)

	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(t.AddDrying) })

	updateSaveState := func() {
		saveBtn.SetSensitive(loaded && !busy)
		if attempted {
			form.Validate(t, spools, false)
		}
	}
	form.spoolDropdown.NotifyProperty("selected", updateSaveState)
	form.tempEntry.NotifyProperty("text", updateSaveState)
	form.durationEntry.NotifyProperty("text", updateSaveState)
	updateSaveState()
	type initial struct {
		spools   []domain.Spool
		profiles []domain.DryingProfile
		err      error
	}
	loader := shared.NewLoader[initial](ctx, dialog)
	form.root.SetSensitive(false)
	var loadInitial func()
	feedback := shared.NewReadFeedback(layout.Get[*gtk.Box](view, "box"), errorLabel, func() { loadInitial() })
	loadInitial = func() {
		feedback.Begin()
		loader.Request(func(ctx context.Context) initial {
			active := false
			spools, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &active})
			if err != nil {
				return initial{err: err}
			}
			p, e := dryingSvc.ListRecommendations(ctx)
			return initial{spools, p, e}
		}, func(data initial) {
			if data.err != nil {
				feedback.Failure(data.err)
				return
			}
			feedback.Success()
			form.root.SetSensitive(true)
			spools = data.spools
			for _, p := range data.profiles {
				profiles[p.Material] = p
			}
			form.SetSpools(t, spools, "")
			applyRecommendation()
			loaded = true
			resetForm()
			updateSaveState()
		})
	}
	loadInitial()
	action := shared.Action(ctx, dialog)
	saveBtn.ConnectClicked(func() {
		attempted = true
		if !form.Validate(t, spools, true) {
			return
		}
		session, ok := form.Session(spools)
		if !ok {
			return
		}
		busy = true
		form.root.SetSensitive(false)
		cancelBtn.SetSensitive(false)
		saveBtn.SetLabel(t.Saving)
		saveBtn.SetSensitive(false)
		action(func() error { return dryingSvc.Create(ctx, session) }, func(err error) {
			busy = false
			form.root.SetSensitive(true)
			cancelBtn.SetSensitive(true)
			saveBtn.SetLabel(t.AddDrying)
			updateSaveState()
			if err != nil {
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
}
