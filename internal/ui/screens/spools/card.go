package spools

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

type spoolCardActions struct {
	dry     func()
	edit    func()
	archive func()
}

func newSpoolCard(t i18n.T, spool domain.Spool, remaining, percent float64, dryingStatus service.DryingStatus, dryingErr error, handlers spoolCardActions, views ...*layout.View) *gtk.Box {
	view := layout.Use("spool-card", views...)

	card := layout.Get[*gtk.Box](view, "card")

	updateSpoolCard(t, view, spool, remaining, percent, dryingStatus, dryingErr)

	dryBtn := spoolActionButton(t.DrySpool, layout.Scope(view, "actions_slot_1"))
	if dryingStatus.NeedsDrying {
		dryBtn.AddCSSClass("warning")
	}
	dryBtn.ConnectClicked(handlers.dry)

	editBtn := spoolActionButton(t.Edit, layout.Scope(view, "actions_slot_2"))
	editBtn.ConnectClicked(handlers.edit)

	archiveBtn := spoolActionButton(t.Archive, layout.Scope(view, "actions_slot_3"))
	archiveBtn.ConnectClicked(handlers.archive)

	return card
}

func spoolActionButton(tooltip string, views ...*layout.View) *gtk.Button {
	view := layout.Use("spool-action", views...)

	button := layout.Get[*gtk.Button](view, "button")

	button.SetTooltipText(tooltip)
	return button
}

func updateSpoolCard(t i18n.T, view *layout.View, spool domain.Spool, remaining, percent float64, dryingStatus service.DryingStatus, dryingErr error) {
	_ = widgets.NewSpoolPreview(spool.ColorHex, layout.Scope(view, "content_slot_1"))
	_ = layout.Configure[*gtk.Label](view, "name", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s — %s", spool.DisplayCode, spool.Manufacturer)) })
	_ = layout.Configure[*gtk.Label](view, "details", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s · %s", spool.Color, spool.Material)) })

	weightText := fmt.Sprintf(t.SpoolWeightFmt, remaining, spool.TotalWeightG, percent)
	_ = widgets.NewLevelProgress(percent, weightText, layout.Scope(view, "card_slot_2"))

	_ = layout.Configure[*gtk.Label](view, "weight", func(w *gtk.Label) { w.SetLabel(weightText) })

	_ = newSpoolDryingSection(t, dryingStatus, dryingErr, layout.Scope(view, "card_slot_4"))

	button := layout.Get[*gtk.Button](layout.Scope(view, "actions_slot_1"), "button")
	button.RemoveCSSClass("warning")
	if dryingStatus.NeedsDrying {
		button.AddCSSClass("warning")
	}
}
