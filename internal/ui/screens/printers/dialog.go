package printers

import (
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
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
	view := layout.Load("printer-dialog")

	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(title)

	closed := false
	dialog.ConnectDestroy(func() { closed = true })

	form := newPrinterForm(t, original, layout.Scope(view, "box_slot_1"))

	statusLabel := layout.Get[*gtk.Label](view, "statusLabel")

	checkButton := layout.Configure[*gtk.Button](view, "checkButton", func(w *gtk.Button) { w.SetLabel(t.CheckConnection) })

	cancelButton := layout.Configure[*gtk.Button](view, "cancelButton", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	cancelButton.ConnectClicked(dialog.Destroy)

	saveButton := layout.Configure[*gtk.Button](view, "saveButton", func(w *gtk.Button) { w.SetLabel(actionLabel) })

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
		shared.RunAsyncUI(ctx, func() {
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
		shared.RunAsyncUI(ctx, func() {
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

	dialog.SetVisible(true)
	form.nameEntry.GrabFocus()
}
