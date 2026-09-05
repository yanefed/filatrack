package drying

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDryingScreen creates the drying history and recommendation screen.
func NewDryingScreen(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), *gtk.Button) {
	t := i18n.L()
	box := widgets.NewScreen(t.Drying)

	box.Append(widgets.NewSectionTitle(t.DryingRecommendations))
	recommendations, refreshRecommendations := newRecommendationsView(t, ctx, dryingSvc)
	box.Append(recommendations)

	box.Append(widgets.NewSectionTitle(t.DryingLog))
	emptyLabel := gtk.NewLabel(t.NoDryingHistory)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)

	historyList := gtk.NewListBox()
	historyList.AddCSSClass("card-list")
	historyList.SetSelectionMode(gtk.SelectionNone)
	box.Append(widgets.NewScrolled(historyList))

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

	addDryingButton := gtk.NewButtonFromIconName("list-add-symbolic")
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
