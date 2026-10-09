package printers

import (
	"context"
	"fmt"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewPrintersScreen creates the Moonraker printer management screen.
func NewPrintersScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func(), *gtk.Button) {
	view := layout.Use("printers-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Printers) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoPrinters) })

	list := layout.Get[*gtk.ListBox](view, "list")

	statusSlots := map[string]*widgets.PrinterStatusWidget{}
	var refreshPrinters func()
	refreshPrinterStatuses := func() {
		printers, err := printerSvc.List(ctx)
		if err != nil {
			return
		}
		for _, printer := range printers {
			status, ok := statusSlots[printer.ID]
			if !ok {
				refreshPrinters()
				return
			}
			status.Update(t, printer)
		}
	}

	refreshPrinters = func() {
		messageLabel.SetVisible(false)
		widgets.ClearListBox(list)
		statusSlots = map[string]*widgets.PrinterStatusWidget{}

		configuredPrinters, err := printerSvc.List(ctx)
		if err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			messageLabel.SetVisible(true)
			emptyLabel.SetVisible(false)
			return
		}
		emptyLabel.SetVisible(len(configuredPrinters) == 0)
		spools := shared.ActiveSpools(ctx, spoolSvc)

		for _, printer := range configuredPrinters {
			card, status := newPrinterCard(t, printer, spools, func(index uint) {
				var err error
				if index == 0 {
					err = printerSvc.ClearAssignedSpool(ctx, printer.ID)
				} else if int(index-1) < len(spools) {
					err = printerSvc.AssignSpool(ctx, printer.ID, spools[index-1].ID)
				}
				if err != nil {
					refreshPrinters()
					messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.AssignPrinterError, err))
					messageLabel.SetVisible(true)
					return
				}
				messageLabel.SetVisible(false)
				if onChanged != nil {
					onChanged()
				}
			}, func() {
				showEditPrinterDialog(ctx, parent, printerSvc, printer, func() {
					refreshPrinters()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			statusSlots[printer.ID] = status
			list.Append(card)
		}
	}

	addPrinterButton := layout.Get[*gtk.Button](view, "addPrinterButton")
	addPrinterButton.SetTooltipText(t.AddPrinter)
	addPrinterButton.ConnectClicked(func() {
		showAddPrinterDialog(ctx, parent, printerSvc, func() {
			refreshPrinters()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	refreshPrinters()
	return box, refreshPrinters, refreshPrinterStatuses, addPrinterButton
}
