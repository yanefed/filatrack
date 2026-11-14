package spools

import (
	"context"
	"fmt"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, dryingSvc *service.DryingService, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func()) {
	view := layout.Use("spools-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Spools) })
	var refreshList func()
	filters := newSpoolFilters(t, func() { refreshList() }, layout.Scope(view, "box_slot_2"))

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoMatchingSpools) })

	spoolGrid := layout.Get[*gtk.GridView](view, "spoolGrid")
	type result struct {
		items   []service.SpoolSummary
		options service.SpoolFilterOptions
		err     error
		full    bool
	}
	loader := shared.NewLoader[result](ctx, box)
	var refreshSpools func()
	updateCards := shared.Virtual(spoolGrid, func(item service.SpoolSummary) string { return item.Spool.ID }, func() (gtk.Widgetter, func(service.SpoolSummary)) {
		cardView := layout.Load("spool-card")
		var current service.SpoolSummary
		archive := shared.Action(ctx, box)
		changed := func() {
			refreshSpools()
			if onChanged != nil {
				onChanged()
			}
		}
		card := newSpoolCard(t, domain.Spool{}, 0, 0, service.DryingStatus{}, nil, spoolCardActions{
			dry: func() {
				item := current
				showDrySpoolDialog(ctx, parent, dryingSvc, item.Spool, item.Drying.Profile, changed)
			},
			edit: func() { item := current; showEditSpoolDialog(ctx, parent, spoolSvc, settingsSvc, item.Spool, changed) },
			archive: func() {
				id := current.Spool.ID
				archive(func() error { return spoolSvc.Archive(ctx, id) }, func(err error) {
					if err != nil {
						emptyLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
						emptyLabel.SetVisible(true)
						return
					}
					changed()
				})
			},
		}, cardView)
		return card, func(item service.SpoolSummary) {
			current = item
			spool := item.Spool
			updateSpoolCard(t, cardView, spool, item.RemainingG, spool.RemainingPercent(spool.TotalWeightG-item.RemainingG), item.Drying, item.DryingErr)
		}
	})
	request := func(full bool) {
		filter := filters.Filter()
		loader.Request(func(ctx context.Context) result {
			data := result{full: full}
			if full {
				data.options, data.err = spoolSvc.FilterOptions(ctx)
				if data.err != nil {
					return data
				}
			}
			if full {
				if filter.Manufacturer != nil && !slices.Contains(data.options.Manufacturers, *filter.Manufacturer) {
					filter.Manufacturer = nil
				}
				if filter.Material != nil && !slices.Contains(data.options.Materials, *filter.Material) {
					filter.Material = nil
				}
			}
			data.items, data.err = spoolSvc.ListSummaries(ctx, filter)
			return data
		}, func(data result) {
			if data.err != nil {
				emptyLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, data.err))
				emptyLabel.SetVisible(true)
				return
			}
			if data.full {
				filters.SetOptions(data.options)
			}
			emptyLabel.SetLabel(t.NoMatchingSpools)
			emptyLabel.SetVisible(len(data.items) == 0)
			updateCards(data.items)
		})
	}
	refreshList = func() { request(false) }
	refreshSpools = func() { request(true) }

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
