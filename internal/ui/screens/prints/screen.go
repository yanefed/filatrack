package prints

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const printsRenderLimit = 250

// NewPrintsScreen создаёт экран истории печатей.
func NewPrintsScreen(ctx context.Context, parent *gtk.Window, printSvc *service.PrintService, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), func()) {
	t := i18n.L()
	box := widgets.NewScreen(t.Prints)

	activeList := newPrintsListBox()

	messageLabel := widgets.NewMessageLabel()
	box.Append(messageLabel)
	box.Append(activeList)

	emptyLabel := gtk.NewLabel(t.NoPrintRecords)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)

	listView, updatePrintsList := newVirtualPrintsList(t, ctx, printSvc, messageLabel)

	box.Append(widgets.NewScrolled(listView))

	var printers []domain.Printer
	activePrintCount := 0
	printerCombo := gtk.NewDropDown(gtk.NewStringList([]string{t.NoPrinters}), nil)
	box.Append(printerCombo)

	renderActivePrints := func() int {
		widgets.ClearListBox(activeList)
		rendered := 0
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
			activeList.Append(widgets.NewPrinterStatus(t, printer))
			rendered++
		}
		return rendered
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
		filter := repository.PrintJobFilter{}
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
		currentSpools := shared.ActiveSpools(ctx, spoolSvc)
		activePrintCount = renderActivePrints()
		updatePrintsList(jobs, currentSpools)
		emptyLabel.SetVisible(activePrintCount == 0 && len(jobs) == 0)
	}

	refreshPrinterChoices := func() {
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

	syncHistoryBtn := gtk.NewButtonWithLabel(t.RefreshHistory)
	box.Append(syncHistoryBtn)
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
	return box, refreshScreen, refreshLivePrints
}

func newPrintsListBox() *gtk.ListBox {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)
	return list
}

func printJobUsageText(t i18n.T, job domain.PrintJob) string {
	printedAt := job.PrintedAt.Format("02.01.2006 15:04")
	if job.ActualWeightG > 0 || job.EstimatedLengthMM <= 0 {
		return fmt.Sprintf(t.UsageFormat, job.ActualWeightG, job.CompletionPercent, printedAt)
	}
	return fmt.Sprintf("Usage: %.0f mm | Completion: %d%% | %s", job.EstimatedLengthMM, job.CompletionPercent, printedAt)
}
