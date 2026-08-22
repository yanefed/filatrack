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

func showAddPrinterDialog(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, onSave func()) {
	t := i18n.L()
	showPrinterDialog(t, ctx, parent, printerSvc, t.AddPrinter, t.AddPrinter, nil, func(printer domain.Printer) error {
		return printerSvc.CreateMoonrakerPrinter(ctx, &printer)
	}, onSave)
}

func showEditPrinterDialog(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, printer domain.Printer, onSave func()) {
	t := i18n.L()
	showPrinterDialog(t, ctx, parent, printerSvc, fmt.Sprintf("%s — %s", t.Edit, printer.Name), t.Save, &printer, func(updated domain.Printer) error {
		return printerSvc.UpdateMoonrakerPrinter(ctx, &updated)
	}, onSave)
}

func showPrinterDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, title, actionLabel string, original *domain.Printer, save func(domain.Printer) error, onSave func()) {
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(title)
	dialog.SetDefaultSize(500, -1)
	closed := false
	dialog.ConnectDestroy(func() { closed = true })

	box := widgets.NewDialogContent(16)
	form := newPrinterForm(t, original)
	box.Append(form.root)

	statusLabel := gtk.NewLabel("")
	statusLabel.SetHAlign(gtk.AlignStart)
	statusLabel.SetWrap(true)
	statusLabel.SetVisible(false)
	box.Append(statusLabel)

	actions := widgets.NewActions(8, gtk.AlignFill)
	checkButton := gtk.NewButtonWithLabel(t.CheckConnection)
	actions.Append(checkButton)
	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	actions.Append(spacer)
	cancelButton := gtk.NewButtonWithLabel(t.Cancel)
	cancelButton.ConnectClicked(dialog.Destroy)
	actions.Append(cancelButton)
	saveButton := gtk.NewButtonWithLabel(actionLabel)
	saveButton.AddCSSClass("suggested-action")
	actions.Append(saveButton)
	box.Append(actions)

	basePrinter := func() domain.Printer {
		if original == nil {
			return domain.Printer{}
		}
		return *original
	}
	updateActions := func() {
		valid := form.Valid()
		checkButton.SetSensitive(valid)
		saveButton.SetSensitive(valid)
	}
	setBusy := func(busy bool) {
		form.root.SetSensitive(!busy)
		cancelButton.SetSensitive(!busy)
		if busy {
			checkButton.SetSensitive(false)
			saveButton.SetSensitive(false)
		} else {
			updateActions()
		}
	}
	invalidateStatus := func() {
		statusLabel.SetVisible(false)
		updateActions()
	}
	form.nameEntry.NotifyProperty("text", invalidateStatus)
	form.urlEntry.NotifyProperty("text", invalidateStatus)
	form.apiKeyEntry.NotifyProperty("text", invalidateStatus)
	updateActions()

	checkButton.ConnectClicked(func() {
		printer := form.Printer(basePrinter())
		setBusy(true)
		statusLabel.RemoveCSSClass("error")
		statusLabel.RemoveCSSClass("success")
		statusLabel.SetLabel("Loading…")
		statusLabel.SetVisible(true)
		var err error
		shared.RunAsyncUI(func() {
			err = printerSvc.CheckMoonrakerConnection(ctx, &printer)
		}, func() {
			if closed {
				return
			}
			setBusy(false)
			if err != nil {
				statusLabel.AddCSSClass("error")
				statusLabel.SetLabel(fmt.Sprintf("%s: %v", t.ConnectionFailed, err))
				return
			}
			statusLabel.AddCSSClass("success")
			statusLabel.SetLabel(t.ConnectionOK)
		})
	})

	saveButton.ConnectClicked(func() {
		printer := form.Printer(basePrinter())
		setBusy(true)
		statusLabel.RemoveCSSClass("error")
		statusLabel.RemoveCSSClass("success")
		statusLabel.SetLabel("Loading…")
		statusLabel.SetVisible(true)
		var err error
		shared.RunAsyncUI(func() {
			err = save(printer)
		}, func() {
			if closed {
				return
			}
			if err != nil {
				setBusy(false)
				statusLabel.AddCSSClass("error")
				statusLabel.SetLabel(fmt.Sprintf("%s: %v", t.AddPrinterError, err))
				return
			}
			dialog.Destroy()
			if onSave != nil {
				onSave()
			}
		})
	})

	dialog.SetChild(box)
	dialog.SetVisible(true)
	form.nameEntry.GrabFocus()
}
