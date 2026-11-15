package models

import (
	"context"
	"path/filepath"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

func showModelUploadDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, messageLabel *gtk.Label, onSuccess func()) {
	view := layout.Load("model-upload-dialog")

	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(t.UploadModel)

	closed := false
	dialog.ConnectDestroy(func() { closed = true })

	var localPath string

	fileRow := layout.Get[*adw.ActionRow](view, "fileRow")

	fileRow.SetTitle(t.NoFileSelected)
	fileRow.SetSubtitle(t.SelectGCode)
	chooseBtn := layout.Configure[*gtk.Button](view, "chooseBtn", func(w *gtk.Button) { w.SetLabel(t.SelectGCode) })

	printerNames := make([]string, len(printers))
	for i, printer := range printers {
		printerNames[i] = widgets.PrinterInfoText(t, printer)
	}
	if len(printerNames) == 0 {
		printerNames = []string{t.NoPrinters}
	}

	printerRow := layout.Get[*adw.ActionRow](view, "printerRow")

	printerRow.SetTitle(t.Printers)
	printerCombo := layout.Configure[*gtk.DropDown](view, "printerCombo", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(printerNames)) })

	startCheck := layout.Get[*gtk.CheckButton](view, "startCheck")

	startRow := layout.Get[*adw.ActionRow](view, "startRow")

	startRow.SetTitle(t.StartAfterUpload)

	spoolNames := make([]string, len(spools))
	for i, spool := range spools {
		spoolNames[i] = shared.SpoolDisplayName(spool)
	}
	if len(spoolNames) == 0 {
		spoolNames = []string{t.NoSpoolsAvail}
	}
	spoolRow := layout.Get[*adw.ActionRow](view, "spoolRow")

	spoolRow.SetTitle(t.SpoolColon)
	spoolCombo := layout.Configure[*gtk.DropDown](view, "spoolCombo", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(spoolNames)) })

	setSpoolFromPrinter := func() {
		printerIndex := printerCombo.Selected()
		if int(printerIndex) >= len(printers) {
			spoolCombo.SetSelected(gtk.InvalidListPosition)
			return
		}
		activeSpoolID := printers[printerIndex].ActiveSpoolID
		for spoolIndex, spool := range spools {
			if spool.ID == activeSpoolID {
				spoolCombo.SetSelected(uint(spoolIndex))
				return
			}
		}
		spoolCombo.SetSelected(gtk.InvalidListPosition)
	}
	setSpoolFromPrinter()
	printerCombo.NotifyProperty("selected", setSpoolFromPrinter)

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	busy, confirmed := false, false
	closeForm, _ := shared.GuardChanges(dialog, func() string { return localPath + shared.FormSnapshot(layout.Get[*gtk.Box](view, "box")) }, func() bool { return busy })
	cancelBtn.ConnectClicked(closeForm)

	uploadBtn := layout.Configure[*gtk.Button](view, "uploadBtn", func(w *gtk.Button) { w.SetLabel(t.UploadModel) })

	content := layout.Get[*gtk.Box](view, "box")
	status := shared.InlineMessage(content)
	content.ReorderChildAfter(status, layout.Get[*gtk.Box](view, "actions"))
	var preview *domain.PrintLaunchResult
	previewLoader := shared.NewLoader[launchPreview](ctx, dialog)
	currentRequest := func() domain.PrintLaunchRequest {
		return domain.PrintLaunchRequest{PrinterID: selectedPrinterID(printerCombo, printers), SpoolID: selectedDirectSpool(spoolCombo, spools), LocalPath: localPath}
	}
	updateState := func() {
		confirmed = false
		preview = nil
		startPrint := startCheck.Active()
		spoolRow.SetVisible(startPrint)
		uploadBtn.SetLabel(t.UploadModel)
		if startPrint {
			uploadBtn.SetLabel(t.UploadAndPrint)
		}
		ready := len(printers) > 0 && localPath != ""
		uploadBtn.SetSensitive(ready && !busy && !startPrint)
		req := currentRequest()
		status.SetVisible(startPrint && ready)
		// Even upload-only selections supersede any pending preview.
		previewLoader.Request(func(c context.Context) launchPreview {
			if !startPrint || !ready {
				return launchPreview{}
			}
			req.AllowLowFilament = settingsSvc.AllowLowFilamentWarning(c)
			result, err := printerSvc.PreviewLaunch(c, req)
			return launchPreview{result, err}
		}, func(data launchPreview) {
			if !startPrint || !ready {
				return
			}
			preview = data.result
			showLaunchResult(t, status, data.result, data.err)
			uploadBtn.SetSensitive(data.err == nil && preview != nil && !preview.Filament.Blocked && !busy)
		})
		if startPrint && ready {
			status.SetText(t.Loading)
		}
	}
	chooseBtn.ConnectClicked(func() {
		showGCodeFileChooser(dialog, func(path string) {
			if closed {
				return
			}
			localPath = path
			fileRow.SetTitle(filepath.Base(path))
			fileRow.SetSubtitle("")
			updateState()
		})
	})
	startCheck.ConnectToggled(updateState)
	printerCombo.NotifyProperty("selected", updateState)
	spoolCombo.NotifyProperty("selected", updateState)
	updateState()
	uploadBtn.ConnectClicked(func() {
		if busy {
			return
		}
		startPrint := startCheck.Active()
		if startPrint && preview == nil {
			return
		}
		if startPrint && preview.RequiresUserConfirmation && !confirmed {
			shared.Confirm(dialog, t.LaunchWarning, filamentText(t, preview.Filament), t.PrintAnyway, func() { confirmed = true; uploadBtn.Object.Emit("clicked") })
			return
		}
		req := currentRequest()
		req.AllowLowFilament = confirmed
		if !startPrint {
			req.SpoolID = ""
		}
		busy = true
		for _, widget := range []gtk.Widgetter{uploadBtn, chooseBtn, printerCombo, spoolCombo, startCheck, cancelBtn} {
			gtk.BaseWidget(widget).SetSensitive(false)
		}
		uploadBtn.SetLabel(t.Uploading)
		status.SetText(t.Uploading)
		status.SetVisible(true)
		var result *domain.PrintLaunchResult
		var err error
		shared.RunAsyncUI(ctx, func() {
			req.AllowLowFilament = req.AllowLowFilament || settingsSvc.AllowLowFilamentWarning(ctx)
			result, err = printerSvc.UploadLocalFile(ctx, req, startPrint)
		}, func() {
			if closed {
				return
			}
			busy = false
			for _, widget := range []gtk.Widgetter{uploadBtn, chooseBtn, printerCombo, spoolCombo, startCheck, cancelBtn} {
				gtk.BaseWidget(widget).SetSensitive(true)
			}
			uploadBtn.SetLabel(t.UploadModel)
			if startPrint {
				uploadBtn.SetLabel(t.UploadAndPrint)
			}
			showLaunchResult(t, status, result, err)
			if err != nil && result != nil && result.LaunchAttempted {
				uploadBtn.SetSensitive(false)
			}
			if result != nil {
				preview = result
			}
			if err == nil && result != nil && (result.Uploaded || result.Started || result.Queued) {
				showLaunchResult(t, messageLabel, result, nil)
				shared.Notify(messageLabel, launchSuccessText(t, result), t.OpenPrinter, func() { shared.OpenPage(messageLabel, "printers") })
				dialog.Destroy()
				if onSuccess != nil {
					onSuccess()
				}
			}
		})
	})

	dialog.SetVisible(true)
}
