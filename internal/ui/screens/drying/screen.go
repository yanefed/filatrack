package drying

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDryingScreen creates the drying history and recommendation screen.
func NewDryingScreen(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), *gtk.Button) {
	view := layout.Use("drying-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Drying) })

	_ = layout.Configure[*gtk.Label](view, "object_512", func(w *gtk.Label) { w.SetLabel(t.DryingRecommendations) })
	_, refreshRecommendations := newRecommendationsView(t, ctx, dryingSvc, layout.Scope(view, "box_slot_3"))

	_ = layout.Configure[*gtk.Label](view, "object_515", func(w *gtk.Label) { w.SetLabel(t.DryingLog) })
	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoDryingHistory) })

	historyList := layout.Get[*gtk.ListBox](view, "historyList")

	refreshDrying := func() {
		refreshRecommendations()
		widgets.ClearListBox(historyList)
		items, err := loadDryingHistory(ctx, dryingSvc, spoolSvc)
		if err != nil {
			emptyLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			emptyLabel.SetVisible(true)
			return
		}
		emptyLabel.SetLabel(t.NoDryingHistory)
		emptyLabel.SetVisible(len(items) == 0)
		for _, item := range items {
			historyList.Append(newDryingHistoryCard(t, item))
		}
	}

	addDryingButton := layout.Get[*gtk.Button](view, "addDryingButton")
	addDryingButton.SetTooltipText(t.AddDrying)
	addDryingButton.ConnectClicked(func() {
		showAddDryingDialog(ctx, parent, dryingSvc, spoolSvc, func() {
			refreshDrying()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	refreshDrying()
	return box, refreshDrying, addDryingButton
}
