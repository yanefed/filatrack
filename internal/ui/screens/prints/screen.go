package prints

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
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

const printsPageSize = 100

// NewPrintsScreen создаёт экран истории печатей.
func NewPrintsScreen(ctx context.Context, parent *gtk.Window, printSvc *service.PrintService, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func(), *gtk.Button, *gtk.DropDown) {
	view := layout.Use("prints-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Prints) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	var printers []domain.Printer
	var choiceIDs []string
	printerCombo := layout.Configure[*gtk.DropDown](view, "printerCombo", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList([]string{t.NoPrinters})) })

	printerCombo.SetTooltipText(t.Printers)

	activeList := newPrintsListBox(layout.Scope(view, "box_slot_3"))

	var activePrinterIDs []string
	var activeStatuses []*widgets.PrinterStatusWidget

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoPrintRecords) })

	_, updatePrintsList := newVirtualPrintsList(t, ctx, printSvc, messageLabel, layout.Scope(view, "object_1087_slot"))

	activePrintCount := 0

	renderActivePrints := func() int {
		visiblePrinters := make([]domain.Printer, 0, len(printers))
		for _, printer := range printers {
			if !widgets.PrinterHasActivePrint(printer) {
				continue
			}
			if len(printers) > 0 {
				idx := printerCombo.Selected()
				if idx > 0 && int(idx) <= len(choiceIDs) && choiceIDs[idx-1] != printer.ID {
					continue
				}
			}
			visiblePrinters = append(visiblePrinters, printer)
		}
		unchanged := len(visiblePrinters) == len(activePrinterIDs)
		if unchanged {
			for i := range visiblePrinters {
				if visiblePrinters[i].ID != activePrinterIDs[i] {
					unchanged = false
					break
				}
			}
		}
		if unchanged {
			for i := range visiblePrinters {
				activeStatuses[i].Update(t, visiblePrinters[i])
			}
			return len(visiblePrinters)
		}

		widgets.ClearListBox(activeList)
		activePrinterIDs = make([]string, 0, len(visiblePrinters))
		activeStatuses = make([]*widgets.PrinterStatusWidget, 0, len(visiblePrinters))
		for _, printer := range visiblePrinters {
			card, status := newActivePrintCard(t, printer)
			activeList.Append(card)
			activePrinterIDs = append(activePrinterIDs, printer.ID)
			activeStatuses = append(activeStatuses, status)
		}
		return len(visiblePrinters)
	}

	type result struct {
		printers []domain.Printer
		spools   []domain.Spool
		jobs     []domain.PrintJob
		err      error
		selected string
		append   bool
		hasMore  bool
	}
	loader := shared.NewLoader[result](ctx, box)
	fullPending, updatingPrinterChoices := false, false
	var choiceNames []string
	var jobs []domain.PrintJob
	var emptyAction *gtk.Button
	loadedCount := printsPageSize
	more := gtk.NewButtonWithLabel(t.ShowMore)
	more.SetVisible(false)
	box.Append(more)
	var refreshPrints func()
	feedback := shared.NewReadFeedback(box, messageLabel, func() { refreshPrints() })
	search := shared.SearchBox(box, func() { loadedCount = printsPageSize; jobs = nil; refreshPrints() })
	selectedID := func() string {
		i := printerCombo.Selected()
		if i > 0 && int(i) <= len(choiceIDs) {
			return choiceIDs[i-1]
		}
		return ""
	}
	request := func(appendPage bool) {
		id, query, limit := selectedID(), search.Text(), loadedCount
		filter := domain.PrintJobFilter{Limit: limit + 1, Search: query}
		if id != "" {
			filter.PrinterID = &id
		}
		if appendPage && len(jobs) > 0 {
			last := jobs[len(jobs)-1]
			filter.Before = &domain.PrintJobCursor{PrintedAt: last.PrintedAt, ID: last.ID}
			filter.Limit = printsPageSize + 1
		}
		if !appendPage && len(jobs) > 0 {
			last := jobs[len(jobs)-1]
			filter.Through = &domain.PrintJobCursor{PrintedAt: last.PrintedAt, ID: last.ID}
			filter.Limit = 0
		}
		fullPending = true
		feedback.Begin()
		more.SetSensitive(false)
		loader.Request(func(c context.Context) result {
			data := result{selected: id, append: appendPage}
			data.printers, data.err = printerSvc.List(c)
			if data.err != nil {
				return data
			}
			data.jobs, data.err = printSvc.List(c, filter)
			if data.err != nil {
				return data
			}
			if filter.Through != nil {
				olderFilter := filter
				olderFilter.Through = nil
				olderFilter.Before = filter.Through
				olderFilter.Limit = 1
				older, err := printSvc.List(c, olderFilter)
				if err != nil {
					data.err = err
					return data
				}
				data.hasMore = len(older) > 0
			}
			data.spools, data.err = spoolSvc.List(c, domain.SpoolFilter{})
			return data
		}, func(data result) {
			fullPending = false
			more.SetSensitive(true)
			if data.err != nil {
				feedback.Failure(data.err)
				return
			}
			updatingPrinterChoices = true
			printers = data.printers
			names, selected := []string{t.AllPrinters}, uint(0)
			choiceIDs = nil
			for i, p := range printers {
				choiceIDs = append(choiceIDs, p.ID)
				names = append(names, p.Name)
				if p.ID == data.selected {
					selected = uint(i + 1)
				}
			}
			if !slices.Equal(choiceNames, names) {
				printerCombo.SetModel(gtk.NewStringList(names))
				choiceNames = names
			}
			printerCombo.SetSelected(selected)
			updatingPrinterChoices = false
			feedback.Success()
			pageLimit := filter.Limit - 1
			hasMore := len(data.jobs) > pageLimit
			if filter.Through != nil {
				hasMore = data.hasMore
				pageLimit = len(data.jobs)
			}
			if hasMore {
				data.jobs = data.jobs[:pageLimit]
			}
			if data.append {
				seen := make(map[string]bool, len(jobs))
				for _, j := range jobs {
					seen[j.ID] = true
				}
				for _, j := range data.jobs {
					if !seen[j.ID] {
						jobs = append(jobs, j)
						seen[j.ID] = true
					}
				}
				loadedCount = max(printsPageSize, len(jobs))
			} else {
				jobs = data.jobs
			}
			more.SetVisible(hasMore)
			activePrintCount = renderActivePrints()
			updatePrintsList(jobs, data.spools)
			emptyLabel.SetLabel(t.NoPrintRecords)
			if query != "" || id != "" {
				emptyLabel.SetLabel(t.NoSearchResults)
			}
			emptyLabel.SetVisible(activePrintCount == 0 && len(jobs) == 0)
			if emptyAction != nil {
				title := t.RefreshHistory
				if len(printers) == 0 {
					title = t.AddPrinter
				}
				if query != "" || id != "" {
					title = t.ResetFilters
				}
				emptyAction.SetLabel(title)
			}
		})
	}
	refreshPrints = func() { request(false) }
	more.ConnectClicked(func() {
		if !fullPending {
			request(true)
		}
	})
	refreshLivePrints := func() {
		if fullPending {
			return
		}
		loader.Poll(func(ctx context.Context) result {
			p, e := printerSvc.List(ctx)
			return result{printers: p, err: e}
		}, func(data result) {
			if data.err != nil {
				return
			}
			printers = data.printers
			previous := activePrintCount
			activePrintCount = renderActivePrints()
			if previous > 0 && activePrintCount == 0 {
				refreshPrints()
			}
		})
	}
	refreshScreen := refreshPrints

	printerCombo.NotifyProperty("selected", func() {
		if !updatingPrinterChoices {
			loadedCount = printsPageSize
			jobs = nil
			refreshPrints()
		}
	})

	syncHistoryBtn := layout.Get[*gtk.Button](view, "syncHistoryBtn")
	syncHistoryBtn.SetTooltipText(t.RefreshHistory)
	syncHistoryBtn.ConnectClicked(func() {
		if len(printers) == 0 {
			return
		}
		printerID := selectedID()
		syncHistoryBtn.SetSensitive(false)
		messageLabel.SetLabel(t.Loading)
		messageLabel.SetVisible(true)
		syncPrinters := append([]domain.Printer(nil), printers...)
		var created int
		var err error
		shared.RunAsyncUI(ctx, func() {
			for _, p := range syncPrinters {
				if printerID != "" && p.ID != printerID {
					continue
				}
				n, syncErr := printerSvc.SyncPrinterHistory(ctx, p.ID, 0)
				created += n
				if syncErr != nil {
					err = syncErr
					break
				}
			}
		}, func() {
			syncHistoryBtn.SetSensitive(true)
			if err != nil {
				messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				messageLabel.SetVisible(true)
				return
			}
			messageLabel.SetLabel(fmt.Sprintf("%s: %d", t.RefreshHistory, created))
			messageLabel.SetVisible(true)
			refreshPrints()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	emptyAction = shared.EmptyAction(ctx, box, emptyLabel, t.RefreshHistory, func() {
		if search.Text() != "" || printerCombo.Selected() != 0 {
			search.SetText("")
			printerCombo.SetSelected(0)
			refreshPrints()
			return
		}
		if len(printers) == 0 {
			shared.OpenPage(box, "printers")
			return
		}
		if shared.ReadOnly(ctx) {
			messageLabel.SetText(t.OfflineAction)
			messageLabel.SetVisible(true)
			return
		}
		syncHistoryBtn.Object.Emit("clicked")
	})
	emptyLabel.SetVisible(false)
	messageLabel.SetText(t.Loading)
	messageLabel.SetVisible(true)
	refreshScreen()
	return box, refreshScreen, refreshLivePrints, syncHistoryBtn, printerCombo
}

func newPrintsListBox(views ...*layout.View) *gtk.ListBox {
	view := layout.Use("active-print-list", views...)

	list := layout.Get[*gtk.ListBox](view, "list")

	return list
}

func newActivePrintCard(t i18n.T, printer domain.Printer) (*gtk.Box, *widgets.PrinterStatusWidget) {
	view := layout.Load("active-print-card")

	card := layout.Get[*gtk.Box](view, "card")

	status := widgets.NewPrinterStatus(t, printer, layout.Scope(view, "card_slot_1"))

	return card, status
}

func printJobUsageText(t i18n.T, job domain.PrintJob) string {
	printedAt := job.PrintedAt.Format("02.01.2006 15:04")
	if job.ActualWeightG > 0 || job.EstimatedLengthMM <= 0 {
		return fmt.Sprintf(t.UsageFormat, job.ActualWeightG, job.CompletionPercent, printedAt)
	}
	return fmt.Sprintf(t.UsageLengthFmt, job.EstimatedLengthMM, job.CompletionPercent, printedAt)
}
