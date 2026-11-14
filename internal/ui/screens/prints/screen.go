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

const printsRenderLimit = 250

// NewPrintsScreen создаёт экран истории печатей.
func NewPrintsScreen(ctx context.Context, parent *gtk.Window, printSvc *service.PrintService, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func(), *gtk.Button, *gtk.DropDown) {
	view := layout.Use("prints-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Prints) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	var printers []domain.Printer
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
				if int(idx) < len(printers) && printers[idx].ID != printer.ID {
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
		live     bool
		selected string
	}
	loader := shared.NewLoader[result](ctx, box)
	fullPending := false
	updatingPrinterChoices := false
	var choiceNames []string
	selectedID := func() string {
		i := printerCombo.Selected()
		if int(i) < len(printers) {
			return printers[i].ID
		}
		return ""
	}
	var refreshPrints func()
	refreshPrints = func() {
		id := selectedID()
		fullPending = true
		loader.Request(func(ctx context.Context) result {
			data := result{selected: id}
			data.printers, data.err = printerSvc.List(ctx)
			if data.err != nil {
				return data
			}
			filter := domain.PrintJobFilter{Limit: printsRenderLimit}
			if id == "" && len(data.printers) > 0 {
				id = data.printers[0].ID
				data.selected = id
			}
			if id != "" {
				found := false
				for _, p := range data.printers {
					if p.ID == id {
						found = true
						break
					}
				}
				if !found && len(data.printers) > 0 {
					id = data.printers[0].ID
					data.selected = id
				}
				if len(data.printers) > 0 {
					filter.PrinterID = &id
				}
			}
			data.jobs, data.err = printSvc.List(ctx, filter)
			if data.err != nil {
				return data
			}
			data.spools, data.err = spoolSvc.List(ctx, domain.SpoolFilter{})
			return data
		}, func(data result) {
			fullPending = false
			if data.err != nil {
				messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, data.err))
				messageLabel.SetVisible(true)
				return
			}
			updatingPrinterChoices = true
			printers = data.printers
			names := make([]string, len(printers))
			selected := uint(0)
			for i, p := range printers {
				names[i] = p.Name
				if p.ID == data.selected {
					selected = uint(i)
				}
			}
			if len(names) == 0 {
				names = []string{t.NoPrinters}
			}
			if !slices.Equal(choiceNames, names) {
				printerCombo.SetModel(gtk.NewStringList(names))
				choiceNames = names
			}
			printerCombo.SetSelected(selected)
			updatingPrinterChoices = false
			messageLabel.SetVisible(false)
			activePrintCount = renderActivePrints()
			updatePrintsList(data.jobs, data.spools)
			emptyLabel.SetVisible(activePrintCount == 0 && len(data.jobs) == 0)
		})
	}
	refreshLivePrints := func() {
		if fullPending {
			return
		}
		loader.Poll(func(ctx context.Context) result {
			p, e := printerSvc.List(ctx)
			return result{printers: p, err: e, live: true}
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
			refreshPrints()
		}
	})

	syncHistoryBtn := layout.Get[*gtk.Button](view, "syncHistoryBtn")
	syncHistoryBtn.SetTooltipText(t.RefreshHistory)
	syncHistoryBtn.ConnectClicked(func() {
		if len(printers) == 0 {
			return
		}
		printerIdx := printerCombo.Selected()
		if int(printerIdx) >= len(printers) {
			return
		}
		printerID := printers[printerIdx].ID
		syncHistoryBtn.SetSensitive(false)
		messageLabel.SetLabel("Loading…")
		messageLabel.SetVisible(true)
		var created int
		var err error
		shared.RunAsyncUI(ctx, func() {
			created, err = printerSvc.SyncPrinterHistory(ctx, printerID, 0)
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
	return fmt.Sprintf("Usage: %.0f mm | Completion: %d%% | %s", job.EstimatedLengthMM, job.CompletionPercent, printedAt)
}
