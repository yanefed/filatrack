package drying

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
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

	status := shared.InlineMessage(box)
	status.SetText(t.Loading)
	status.SetVisible(true)
	emptyLabel.SetVisible(false)
	historyList := layout.Get[*gtk.ListView](view, "historyList")
	updateHistory := shared.Virtual(historyList, func(item dryingHistoryItem) string { return item.session.ID }, func() (gtk.Widgetter, func(dryingHistoryItem)) {
		rowView := layout.Load("drying-history-card")
		return layout.Get[*gtk.Box](rowView, "card"), func(item dryingHistoryItem) { newDryingHistoryCard(t, item, rowView) }
	})
	type result struct {
		items []dryingHistoryItem
		err   error
	}
	loader := shared.NewLoader[result](ctx, box)
	var refreshDrying func()
	feedback := shared.NewReadFeedback(box, status, func() { refreshDrying() })
	refreshDrying = func() {
		feedback.Begin()
		refreshRecommendations()
		loader.Request(func(ctx context.Context) result {
			items, err := loadDryingHistory(ctx, dryingSvc, spoolSvc)
			return result{items, err}
		}, func(data result) {
			if data.err != nil {
				feedback.Failure(data.err)
				return
			}
			feedback.Success()
			emptyLabel.SetLabel(t.NoDryingHistory)
			emptyLabel.SetVisible(len(data.items) == 0)
			updateHistory(data.items)
		})
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

	shared.ProtectWrites(ctx, shared.EmptyAction(ctx, box, emptyLabel, t.AddDrying, func() { addDryingButton.Object.Emit("clicked") }))
	refreshDrying()
	return box, refreshDrying, addDryingButton
}
