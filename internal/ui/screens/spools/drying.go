package spools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
)

func newSpoolDryingSection(t i18n.T, status service.DryingStatus, statusErr error, views ...*layout.View) *gtk.Box {
	view := layout.Use("spool-drying-status", views...)

	section := layout.Get[*gtk.Box](view, "section")

	lastText := t.NeverDried
	if status.LastSession != nil {
		lastText = fmt.Sprintf(t.LastDryingFmt, status.LastSession.DriedAt.Format("02.01.2006"))
	}
	_ = dryingCaption(lastText, layout.Scope(view, "section_slot_2"))

	stateLabel := dryingCaption(dryingStateText(t, status, statusErr), layout.Scope(view, "section_slot_3"))
	stateLabel.AddCSSClass("dim-label")
	stateLabel.RemoveCSSClass("drying-warning")
	if status.NeedsDrying {
		stateLabel.RemoveCSSClass("dim-label")
		stateLabel.AddCSSClass("drying-warning")
	}

	return section
}

func dryingCaption(text string, views ...*layout.View) *gtk.Label {
	view := layout.Use("drying-caption", views...)

	label := layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(text) })

	return label
}

func dryingStateText(t i18n.T, status service.DryingStatus, statusErr error) string {
	if statusErr != nil {
		return fmt.Sprintf("%s: %v", t.ErrorPrefix, statusErr)
	}
	if status.Profile == nil {
		return t.NoDryingProfile
	}
	if status.LastSession == nil {
		return t.DryingRequired
	}
	if status.NeedsDrying {
		if status.DaysUntilNext < 0 {
			return fmt.Sprintf(t.DryingOverdueFmt, -status.DaysUntilNext)
		}
		return t.DryingRequired
	}
	if status.DaysUntilNext == 0 {
		return t.DryingDueToday
	}
	return fmt.Sprintf(t.NextDryingFmt, status.DaysUntilNext)
}

func showDrySpoolDialog(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spool domain.Spool, profile *domain.DryingProfile, onSave func()) {
	view := layout.Load("dry-spool-dialog")

	t := i18n.L()
	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(fmt.Sprintf("%s", t.DrySpool))
	_ = layout.Configure[*gtk.Label](view, "spoolLabel", func(w *gtk.Label) {
		w.SetLabel(fmt.Sprintf("%s — %s %s (%s)", spool.DisplayCode, spool.Manufacturer, spool.Color, spool.Material))
	})
	_ = layout.Configure[*gtk.Label](view, "temperatureLabel", func(w *gtk.Label) { w.SetLabel(t.TemperatureC) })

	temperatureEntry := layout.Get[*gtk.Entry](view, "temperatureEntry")
	temperatureEntry.SetPlaceholderText(t.TemperatureC)
	_ = layout.Configure[*gtk.Label](view, "durationLabel", func(w *gtk.Label) { w.SetLabel(t.DurationHours) })

	durationEntry := layout.Get[*gtk.Entry](view, "durationEntry")
	durationEntry.SetPlaceholderText(t.DurationHours)

	if profile != nil {
		temperatureEntry.SetText(strconv.Itoa(profile.TemperatureC))
		durationEntry.SetText(strconv.FormatFloat(profile.DurationHours, 'f', -1, 64))
	}

	errorLabel := layout.Get[*gtk.Label](view, "errorLabel")

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	busy := false
	closeForm, _ := shared.GuardChanges(dialog, func() string { return temperatureEntry.Text() + "\x00" + durationEntry.Text() }, func() bool { return busy })
	cancelBtn.ConnectClicked(closeForm)

	saveBtn := layout.Configure[*gtk.Button](view, "saveBtn", func(w *gtk.Button) { w.SetLabel(t.Save) })

	action := shared.Action(ctx, dialog)
	saveBtn.ConnectClicked(func() {

		temperature, te := strconv.Atoi(strings.TrimSpace(temperatureEntry.Text()))
		duration, de := shared.ParseDecimal(durationEntry.Text())
		temperatureEntry.RemoveCSSClass("error")
		durationEntry.RemoveCSSClass("error")
		if te != nil || temperature <= 0 {
			errorLabel.SetText(t.TemperatureC + ": " + t.InvalidPositive)
			errorLabel.SetVisible(true)
			temperatureEntry.AddCSSClass("error")
			temperatureEntry.GrabFocus()
			return
		}
		if de != nil || duration <= 0 {
			errorLabel.SetText(t.DurationHours + ": " + t.InvalidPositive)
			errorLabel.SetVisible(true)
			durationEntry.AddCSSClass("error")
			durationEntry.GrabFocus()
			return
		}

		session := &domain.DryingSession{
			SpoolID:       spool.ID,
			TemperatureC:  temperature,
			DurationHours: duration,
		}
		busy = true
		temperatureEntry.SetSensitive(false)
		durationEntry.SetSensitive(false)
		cancelBtn.SetSensitive(false)
		saveBtn.SetLabel(t.Saving)
		saveBtn.SetSensitive(false)
		action(func() error { return dryingSvc.Create(ctx, session) }, func(err error) {
			busy = false
			temperatureEntry.SetSensitive(true)
			durationEntry.SetSensitive(true)
			cancelBtn.SetSensitive(true)
			saveBtn.SetLabel(t.Save)
			saveBtn.SetSensitive(true)
			if err != nil {
				errorLabel.SetText(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
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
