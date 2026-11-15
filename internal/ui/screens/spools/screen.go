package spools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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

	archivedCombo := gtk.NewDropDown(gtk.NewStringList([]string{t.ActiveSpools, t.ArchivedSpools}), nil)
	box.InsertChildAfter(archivedCombo, layout.Get[*gtk.Label](view, "box_title"))
	status := shared.InlineMessage(box)

	emptyLabel.SetVisible(false)

	var search *gtk.SearchEntry
	var cached []service.SpoolSummary
	var render func()
	var resetEmpty *gtk.Button
	search = shared.SearchBox(box, func() {
		if render != nil {
			render()
		}
	})
	spoolGrid := layout.Get[*gtk.GridView](view, "spoolGrid")
	type result struct {
		items   []service.SpoolSummary
		options service.SpoolFilterOptions
		err     error
		full    bool
	}
	loader := shared.NewLoader[result](ctx, box)
	var refreshSpools func()
	readStatus := shared.InlineMessage(box)
	box.ReorderChildAfter(readStatus, filters.root)
	box.ReorderChildAfter(status, readStatus)
	feedback := shared.NewReadFeedback(box, readStatus, func() { refreshSpools() })
	status.SetVisible(false)
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
				spool := current.Spool
				if spool.Archived {
					archive(func() error { _, err := spoolSvc.Restore(ctx, domain.SpoolArchive{SpoolID: spool.ID}); return err }, func(err error) {
						if err != nil {
							status.SetText(archiveError(t, err))
							status.SetVisible(true)
							return
						}
						changed()
						shared.Notify(box, t.SpoolRestored, "", nil)
					})
					return
				}
				var perform func([]string)
				perform = func(detachPrinterIDs []string) {
					var receipt domain.SpoolArchive
					archive(func() error {
						var err error
						receipt, err = spoolSvc.ArchiveWithAssignments(ctx, spool.ID, detachPrinterIDs)
						return err
					}, func(err error) {
						if errors.Is(err, domain.ErrSpoolAssigned) {
							names := make([]string, len(receipt.Printers))
							for i, p := range receipt.Printers {
								names[i] = p.Name
							}
							shared.Confirm(parent, t.Archive, fmt.Sprintf(t.ArchiveDetachFmt, strings.Join(names, "\n")), t.Archive, func() {
								ids := make([]string, len(receipt.Printers))
								for i, p := range receipt.Printers {
									ids[i] = p.ID
								}
								perform(ids)
							})
							return
						}
						if err != nil {
							status.SetText(archiveError(t, err))
							status.SetVisible(true)
							return
						}
						changed()
						shared.Notify(box, t.SpoolArchived, t.Undo, func() {
							var restored domain.SpoolRestoreResult
							archive(func() error { var err error; restored, err = spoolSvc.Restore(ctx, receipt); return err }, func(err error) {
								if err != nil {
									status.SetText(archiveError(t, err))
									status.SetVisible(true)
									return
								}
								changed()
								text := t.SpoolRestored
								if len(restored.SkippedPrinterIDs) > 0 {
									text = t.RestoreAssignmentsSkipped
								}
								shared.Notify(box, text, "", nil)
							})
						})
					})
				}
				perform(nil)
			},
		}, cardView)
		for _, slot := range []string{"actions_slot_1", "actions_slot_2", "actions_slot_3"} {
			shared.ProtectWrites(ctx, layout.Get[*gtk.Button](layout.Scope(cardView, slot), "button"))
		}
		return card, func(item service.SpoolSummary) {
			current = item
			spool := item.Spool
			updateSpoolCard(t, cardView, spool, item.RemainingG, spool.RemainingPercent(spool.TotalWeightG-item.RemainingG), item.Drying, item.DryingErr)
		}
	})

	render = func() {
		query := strings.ToLower(strings.TrimSpace(search.Text()))
		visible := make([]service.SpoolSummary, 0, len(cached))
		for _, item := range cached {
			spool := item.Spool
			if query != "" && !strings.Contains(strings.ToLower(spool.DisplayCode+" "+spool.Manufacturer+" "+string(spool.Material)+" "+spool.Color), query) {
				continue
			}
			visible = append(visible, item)
		}
		f := filters.Filter()
		emptyLabel.SetLabel(t.EmptySpools)
		if archivedCombo.Selected() == 1 {
			emptyLabel.SetLabel(t.EmptyArchive)
		}
		if query != "" || f.Manufacturer != nil || f.Material != nil {
			emptyLabel.SetLabel(t.NoSearchResults)
		}
		emptyLabel.SetVisible(len(visible) == 0)
		if resetEmpty != nil {
			f := filters.Filter()
			resetEmpty.SetVisible(len(visible) == 0 && (query != "" || archivedCombo.Selected() == 1 || f.Material != nil || f.Manufacturer != nil))
		}
		updateCards(visible)
	}
	request := func(full bool) {
		feedback.Begin()
		filter := filters.Filter()
		archived := archivedCombo.Selected() == 1
		filter.Archived = &archived
		loader.Request(func(ctx context.Context) result {
			data := result{full: full}
			if full {
				data.options, data.err = spoolSvc.FilterOptionsFor(ctx, archived)
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
				feedback.Failure(data.err)
				return
			}
			if data.full {
				filters.SetOptions(data.options)
			}
			feedback.Success()
			cached = data.items
			render()
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

	resetEmpty = shared.EmptyAction(ctx, box, emptyLabel, t.ResetFilters, func() {
		search.SetText("")
		filters.reset.Object.Emit("clicked")
		archivedCombo.SetSelected(0)
	})
	addEmpty := shared.EmptyAction(ctx, box, emptyLabel, t.AddSpoolBtn, openAddDialog)
	addEmpty.SetTooltipText(t.AddSpoolBtn)
	shared.ProtectWrites(ctx, addEmpty)
	archivedCombo.NotifyProperty("selected", refreshSpools)
	refreshSpools()
	return box, refreshSpools, openAddDialog
}

func archiveError(t i18n.T, err error) string {
	if errors.Is(err, domain.ErrSpoolInUse) {
		return t.ArchiveInUse
	}
	if errors.Is(err, domain.ErrArchiveChanged) {
		return t.ArchiveChanged
	}
	return fmt.Sprintf("%s: %v", t.ErrorPrefix, err)
}
