package printers

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewPrintersScreen creates the Moonraker printer management screen.
func NewPrintersScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), func(), *gtk.Button) {
	t := i18n.L()
	box := widgets.NewScreen(t.Printers)

	messageLabel := widgets.NewMessageLabel()
	box.Append(messageLabel)
	emptyLabel := gtk.NewLabel(t.NoPrinters)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)

	list := gtk.NewListBox()
	list.AddCSSClass("card-list")
	list.SetSelectionMode(gtk.SelectionNone)
	box.Append(widgets.NewScrolled(list))

	statusSlots := map[string]*gtk.Box{}
	var refreshPrinters func()
	refreshPrinterStatuses := func() {
		printers, err := printerSvc.List(ctx)
		if err != nil {
			return
		}
		for _, printer := range printers {
			slot, ok := statusSlots[printer.ID]
			if !ok {
				refreshPrinters()
				return
			}
			widgets.ReplaceBoxChild(slot, widgets.NewPrinterStatus(t, printer))
		}
	}

	refreshPrinters = func() {
		messageLabel.SetVisible(false)
		widgets.ClearListBox(list)
		statusSlots = map[string]*gtk.Box{}

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
			card, statusSlot := newPrinterCard(t, printer, spools, func(index uint) {
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
			statusSlots[printer.ID] = statusSlot
			list.Append(card)
		}
	}

	addPrinterButton := gtk.NewButtonFromIconName("list-add-symbolic")
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
