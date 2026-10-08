package spools

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, dryingSvc *service.DryingService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func(), func()) {
	t := i18n.L()
	box := widgets.NewScreen(t.Spools)
	var refreshList func()
	filters := newSpoolFilters(t, func() { refreshList() })
	box.Append(filters.root)

	emptyLabel := gtk.NewLabel(t.NoMatchingSpools)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)

	spoolGrid := gtk.NewFlowBox()
	spoolGrid.SetVAlign(gtk.AlignStart)
	spoolGrid.SetSelectionMode(gtk.SelectionNone)
	spoolGrid.SetHomogeneous(true)
	spoolGrid.SetMinChildrenPerLine(1)
	spoolGrid.SetMaxChildrenPerLine(3)
	spoolGrid.SetColumnSpacing(12)
	spoolGrid.SetRowSpacing(12)

	scrolled := widgets.NewScrolled(spoolGrid)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	box.Append(scrolled)

	var refreshSpools func()
	refreshList = func() {
		spoolGrid.RemoveAll()
		emptyLabel.SetVisible(false)

		spools, err := spoolSvc.List(ctx, filters.Filter())
		if err != nil {
			label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
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
			spoolGrid.RemoveAll()
			emptyLabel.SetVisible(false)
			spoolGrid.Append(gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err)))
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
