package spools

import (
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, dryingSvc *service.DryingService, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func()) {
	view := layout.Use("spools-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Spools) })
	var refreshList func()
	filters := newSpoolFilters(t, func() { refreshList() }, layout.Scope(view, "box_slot_2"))

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoMatchingSpools) })

	spoolGrid := layout.Get[*gtk.FlowBox](view, "spoolGrid")

	var refreshSpools func()
	refreshList = func() {
		spoolGrid.RemoveAll()
		emptyLabel.SetVisible(false)

		spools, err := spoolSvc.List(ctx, filters.Filter())
		if err != nil {
			view := layout.Load("spools-error")

			label := layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err)) })
			spoolGrid.Append(label)
			return
		}
		emptyLabel.SetVisible(len(spools) == 0)

		for _, spool := range spools {
			_, remaining, err := spoolSvc.GetWithRemaining(ctx, spool.ID)
			if err != nil {
				remaining = 0
			}
			percent := spool.RemainingPercent(spool.TotalWeightG - remaining)
			dryingStatus, dryingErr := dryingSvc.GetStatus(ctx, &spool)

			spoolGrid.Append(newSpoolCard(t, spool, remaining, percent, dryingStatus, dryingErr, spoolCardActions{
				dry: func() {
					showDrySpoolDialog(ctx, parent, dryingSvc, spool, dryingStatus.Profile, func() {
						refreshSpools()
						if onChanged != nil {
							onChanged()
						}
					})
				},
				edit: func() {
					showEditSpoolDialog(ctx, parent, spoolSvc, settingsSvc, spool, func() {
						refreshSpools()
						if onChanged != nil {
							onChanged()
						}
					})
				},
				archive: func() {
					if err := spoolSvc.Archive(ctx, spool.ID); err != nil {
						return
					}
					refreshSpools()
					if onChanged != nil {
						onChanged()
					}
				},
			}))
		}
	}
	refreshSpools = func() {
		options, err := spoolSvc.FilterOptions(ctx)
		if err != nil {
			view := layout.Load("spools-filter-error")

			spoolGrid.RemoveAll()
			emptyLabel.SetVisible(false)
			spoolGrid.Append(layout.Configure[*gtk.Label](view, "object_1611", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err)) }))
			return
		}
		filters.SetOptions(options)
		refreshList()
	}

	openAddDialog := func() {
		showAddSpoolDialog(ctx, parent, spoolSvc, settingsSvc, func() {
			refreshSpools()
			if onChanged != nil {
				onChanged()
			}
		})
	}

	refreshSpools()
	return box, refreshSpools, openAddDialog
}
