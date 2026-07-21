package screens

import (
	"context"
	"fmt"
	"strings"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewPrintersScreen creates the Moonraker printer management screen.
func NewPrintersScreen(ctx context.Context, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), func()) {
	t := i18n.L()

	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	title := gtk.NewLabel(t.Printers)
	title.AddCSSClass("title-1")
	box.Append(title)

	notice := gtk.NewLabel(t.PrinterMonitorNotice)
	notice.SetHAlign(gtk.AlignStart)
	notice.AddCSSClass("dim-label")
	notice.SetWrap(true)
	box.Append(notice)

	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(list)
	box.Append(scrolled)

	errorLabel := gtk.NewLabel("")
	errorLabel.SetHAlign(gtk.AlignStart)
	errorLabel.AddCSSClass("dim-label")
	errorLabel.SetWrap(true)
	errorLabel.SetVisible(false)
	box.Append(errorLabel)

	addForm := gtk.NewBox(gtk.OrientationVertical, 6)
	addForm.SetMarginTop(8)

	nameEntry := gtk.NewEntry()
	nameEntry.SetPlaceholderText(t.PrinterName)
	addForm.Append(nameEntry)

	urlEntry := gtk.NewEntry()
	urlEntry.SetPlaceholderText(t.MoonrakerURL)
	addForm.Append(urlEntry)

	apiKeyEntry := gtk.NewEntry()
	apiKeyEntry.SetPlaceholderText(t.APIKey)
	apiKeyEntry.SetVisibility(false)
	addForm.Append(apiKeyEntry)

	addBtn := gtk.NewButtonWithLabel(t.AddPrinter)
	addForm.Append(addBtn)

	checkBtn := gtk.NewButtonWithLabel(t.CheckConnection)
	addForm.Append(checkBtn)
	box.Append(addForm)

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
			replaceBoxChild(slot, printerStatusWidget(t, printer))
		}
	}
	refreshPrinters = func() {
		errorLabel.SetVisible(false)
		clearListBox(list)
		statusSlots = map[string]*gtk.Box{}

		printers, err := printerSvc.List(ctx)
		if err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			errorLabel.SetVisible(true)
			return
		}

		spools := activeSpools(ctx, spoolSvc)
		if len(printers) == 0 {
			empty := gtk.NewLabel(t.NoPrinters)
			empty.AddCSSClass("dim-label")
			list.Append(empty)
			return
		}

		for _, printer := range printers {
			printer := printer
			row := gtk.NewBox(gtk.OrientationVertical, 4)
			row.SetMarginTop(6)
			row.SetMarginBottom(8)

			statusSlot := gtk.NewBox(gtk.OrientationVertical, 0)
			statusSlot.Append(printerStatusWidget(t, printer))
			statusSlots[printer.ID] = statusSlot
			row.Append(statusSlot)

			assignRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
			assignLabel := gtk.NewLabel(t.ActiveSpool)
			assignLabel.SetHAlign(gtk.AlignStart)
			assignRow.Append(assignLabel)

			spoolNames := spoolChoiceNames(t, spools)
			dropdown := gtk.NewDropDown(gtk.NewStringList(spoolNames), nil)
			selected := selectedSpoolIndex(printer.ActiveSpoolID, spools)
			if pending, ok := pendingSpoolSelection[printer.ID]; ok && int(pending) <= len(spools) {
				selected = pending
			}
			dropdown.SetSelected(selected)
			dropdown.NotifyProperty("selected", func() {
				pendingSpoolSelection[printer.ID] = dropdown.Selected()
			})
			assignRow.Append(dropdown)

			saveBtn := gtk.NewButtonWithLabel(t.SaveAssignment)
			saveBtn.ConnectClicked(func() {
				idx := dropdown.Selected()
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
			})
			assignRow.Append(saveBtn)
			row.Append(assignRow)

			list.Append(row)
		}
	}

	addBtn.ConnectClicked(func() {
		printer := &domain.Printer{
			Name:         nameEntry.Text(),
			MoonrakerURL: urlEntry.Text(),
			APIKey:       strings.TrimSpace(apiKeyEntry.Text()),
		}
		if err := printerSvc.CreateMoonrakerPrinter(ctx, printer); err != nil {
			errorLabel.SetLabel(fmt.Sprintf("%s: %v", t.AddPrinterError, err))
			errorLabel.SetVisible(true)
			return
		}
		nameEntry.SetText("")
		urlEntry.SetText("")
		apiKeyEntry.SetText("")
		refreshPrinters()
		if onChanged != nil {
			onChanged()
		}
	})

	checkBtn.ConnectClicked(func() {
		printer := &domain.Printer{
			Name:         nameEntry.Text(),
			MoonrakerURL: urlEntry.Text(),
			APIKey:       strings.TrimSpace(apiKeyEntry.Text()),
		}
		if err := printerSvc.CheckMoonrakerConnection(ctx, printer); err != nil {
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
