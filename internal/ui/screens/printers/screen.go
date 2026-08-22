package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewPrintersScreen creates the Moonraker printer management screen.
func NewPrintersScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), func()) {
	t := i18n.L()

	box := widgets.NewScreen(t.Printers)

	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)

	box.Append(widgets.NewScrolled(list))

	errorLabel := widgets.NewMessageLabel()
	box.Append(errorLabel)

	addForm := newPrinterForm(t)

	addBtn := gtk.NewButtonWithLabel(t.AddPrinter)
	addForm.root.Append(addBtn)

	checkBtn := gtk.NewButtonWithLabel(t.CheckConnection)
	addForm.root.Append(checkBtn)
	box.Append(addForm.root)

	pendingSpoolSelection := map[string]uint{}
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
		errorLabel.SetVisible(false)
		widgets.ClearListBox(list)
		statusSlots = map[string]*gtk.Box{}

		printers, err := printerSvc.List(ctx)
		if err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
			return
		}

		spools := shared.ActiveSpools(ctx, spoolSvc)
		if len(printers) == 0 {
			empty := gtk.NewLabel(t.NoPrinters)
			empty.AddCSSClass("dim-label")
			list.Append(empty)
			return
		}

		for _, printer := range printers {
			selected := shared.SelectedSpoolIndex(printer.ActiveSpoolID, spools)
			if pending, ok := pendingSpoolSelection[printer.ID]; ok && int(pending) <= len(spools) {
				selected = pending
			}
			row, statusSlot := newPrinterRow(t, printer, spools, selected, func(selected uint) {
				pendingSpoolSelection[printer.ID] = selected
			}, func(idx uint) {
				var err error
				if idx == 0 {
					err = printerSvc.ClearAssignedSpool(ctx, printer.ID)
				} else if int(idx-1) < len(spools) {
					err = printerSvc.AssignSpool(ctx, printer.ID, spools[idx-1].ID)
				}
				if err != nil {
					errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.AssignPrinterError, err))
					errorLabel.SetVisible(true)
					return
				}
				delete(pendingSpoolSelection, printer.ID)
				errorLabel.SetVisible(false)
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
			list.Append(row)
		}
	}

	addBtn.ConnectClicked(func() {
		printer := addForm.Printer()
		if err := printerSvc.CreateMoonrakerPrinter(ctx, &printer); err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.AddPrinterError, err))
			errorLabel.SetVisible(true)
			return
		}
		addForm.Reset()
		refreshPrinters()
		if onChanged != nil {
			onChanged()
		}
	})

	checkBtn.ConnectClicked(func() {
		printer := addForm.Printer()
		if err := printerSvc.CheckMoonrakerConnection(ctx, &printer); err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ConnectionFailed, err))
			errorLabel.SetVisible(true)
			return
		}
		errorLabel.SetLabel(t.ConnectionOK)
		errorLabel.SetVisible(true)
	})

	refreshPrinters()
	return box, refreshPrinters, refreshPrinterStatuses
}

func showEditPrinterDialog(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, printer domain.Printer, onSave func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s — %s", t.Edit, printer.Name))
	dialog.SetDefaultSize(440, 260)

	box := widgets.NewDialogContent(8)

	nameEntry := gtk.NewEntry()
	nameEntry.SetPlaceholderText(t.PrinterName)
	nameEntry.SetText(printer.Name)
	box.Append(nameEntry)

	urlEntry := gtk.NewEntry()
	urlEntry.SetPlaceholderText(t.MoonrakerURL)
	urlEntry.SetText(printer.MoonrakerURL)
	box.Append(urlEntry)

	statusLabel := gtk.NewLabel("")
	statusLabel.SetHAlign(gtk.AlignStart)
	statusLabel.SetWrap(true)
	statusLabel.AddCSSClass("dim-label")
	statusLabel.SetVisible(false)
	box.Append(statusLabel)

	actions := widgets.NewActions(8, gtk.AlignFill)
	checkBtn := gtk.NewButtonWithLabel(t.CheckConnection)
	saveBtn := gtk.NewButtonWithLabel(t.Save)
	saveBtn.SetSensitive(false)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	actions.Append(checkBtn)
	actions.Append(saveBtn)
	actions.Append(cancelBtn)
	box.Append(actions)

	updatedPrinter := func() domain.Printer {
		updated := printer
		updated.Name = nameEntry.Text()
		updated.MoonrakerURL = urlEntry.Text()
		return updated
	}
	invalidateCheck := func() {
		saveBtn.SetSensitive(false)
		statusLabel.SetVisible(false)
	}
	nameEntry.NotifyProperty("text", invalidateCheck)
	urlEntry.NotifyProperty("text", invalidateCheck)

	checkBtn.ConnectClicked(func() {
		updated := updatedPrinter()
		if err := printerSvc.CheckMoonrakerConnection(ctx, &updated); err != nil {
			statusLabel.SetLabel(fmt.Sprintf("%s: %v", t.ConnectionFailed, err))
			statusLabel.SetVisible(true)
			saveBtn.SetSensitive(false)
			return
		}
		statusLabel.SetLabel(t.ConnectionOK)
		statusLabel.SetVisible(true)
		saveBtn.SetSensitive(true)
	})

	saveBtn.ConnectClicked(func() {
		updated := updatedPrinter()
		if err := printerSvc.UpdateMoonrakerPrinter(ctx, &updated); err != nil {
			statusLabel.SetLabel(fmt.Sprintf("%s: %v", t.AddPrinterError, err))
			statusLabel.SetVisible(true)
			saveBtn.SetSensitive(false)
			return
		}
		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	})
	cancelBtn.ConnectClicked(dialog.Destroy)

	dialog.SetChild(box)
	dialog.SetVisible(true)
}
