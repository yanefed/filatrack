package prints

import (
	"context"
	"fmt"

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

	var refreshPrints func()
	refreshLivePrints := func() {
		loaded, err := printerSvc.List(ctx)
		if err != nil {
			return
		}
		printers = loaded
		previousActiveCount := activePrintCount
		activePrintCount = renderActivePrints()
		if previousActiveCount > 0 && activePrintCount == 0 {
			refreshPrints()
		}
	}
	refreshPrints = func() {
		filter := domain.PrintJobFilter{}
		if len(printers) > 0 {
			idx := printerCombo.Selected()
			if int(idx) < len(printers) {
				printerID := printers[idx].ID
				filter.PrinterID = &printerID
			}
		}
		filter.Limit = printsRenderLimit
		jobs, err := printSvc.List(ctx, filter)
		if err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			messageLabel.SetVisible(true)
			updatePrintsList(nil, nil)
			emptyLabel.SetVisible(false)
			return
		}
		// Все катушки, включая архивированные: записи истории могут ссылаться на них.
		currentSpools := shared.AllSpools(ctx, spoolSvc)
		activePrintCount = renderActivePrints()
		updatePrintsList(jobs, currentSpools)
		emptyLabel.SetVisible(activePrintCount == 0 && len(jobs) == 0)
	}

	updatingPrinterChoices := false
	refreshPrinterChoices := func() {
		updatingPrinterChoices = true
		defer func() { updatingPrinterChoices = false }()
		currentID := ""
		if len(printers) > 0 {
			selected := printerCombo.Selected()
			if int(selected) < len(printers) {
				currentID = printers[selected].ID
			}
		}

		loaded, err := printerSvc.List(ctx)
		if err != nil {
			printers = nil
			printerCombo.SetModel(gtk.NewStringList([]string{fmt.Sprintf("%s: %v", t.ErrorPrefix, err)}))
			printerCombo.SetSelected(0)
			return
		}

		printers = loaded
		printerNames := make([]string, len(printers))
		selectedIdx := uint(0)
		for i, printer := range printers {
			printerNames[i] = printer.Name
			if printer.ID == currentID {
				selectedIdx = uint(i)
			}
		}
		if len(printerNames) == 0 {
			printerNames = []string{t.NoPrinters}
		}
		printerCombo.SetModel(gtk.NewStringList(printerNames))
		printerCombo.SetSelected(selectedIdx)
		activePrintCount = renderActivePrints()
	}

	refreshScreen := func() {
		refreshPrinterChoices()
		refreshPrints()
	}
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
		shared.RunAsyncUI(func() {
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
