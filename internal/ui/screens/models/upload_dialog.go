package models

import (
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"

	"context"
	"path/filepath"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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
		printerNames[i] = printer.Name
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
	cancelBtn.ConnectClicked(dialog.Destroy)

	uploadBtn := layout.Configure[*gtk.Button](view, "uploadBtn", func(w *gtk.Button) { w.SetLabel(t.UploadModel) })

	updateState := func() {
		startPrint := startCheck.Active()
		spoolRow.SetVisible(startPrint)
		if startPrint {
			uploadBtn.SetLabel(t.UploadAndPrint)
		} else {
			uploadBtn.SetLabel(t.UploadModel)
		}
		uploadBtn.SetSensitive(len(printers) > 0 && localPath != "")
	}
	chooseBtn.ConnectClicked(func() {
		showGCodeFileChooser(dialog, func(path string) {
			localPath = path
			fileRow.SetTitle(filepath.Base(path))
			fileRow.SetSubtitle("")
			updateState()
		})
	})
	startCheck.ConnectToggled(updateState)
	updateState()

	uploadBtn.ConnectClicked(func() {
		printerID := selectedPrinterID(printerCombo, printers)
		if printerID == "" {
			return
		}
		startPrint := startCheck.Active()
		spoolID := ""
		if startPrint {
			spoolID = selectedDirectSpool(spoolCombo, spools)
		}
		req := domain.PrintLaunchRequest{
			PrinterID:        printerID,
			SpoolID:          spoolID,
			LocalPath:        localPath,
			AllowLowFilament: settingsSvc.AllowLowFilamentWarning(ctx),
		}
		uploadBtn.SetSensitive(false)
		chooseBtn.SetSensitive(false)
		messageLabel.SetLabel("Loading…")
		messageLabel.SetVisible(true)
		var result *domain.PrintLaunchResult
		var err error
		shared.RunAsyncUI(func() {
			if startPrint && spoolID != "" {
				for i := range printers {
					if printers[i].ID == printerID && printers[i].ActiveSpoolID != spoolID {
						err = printerSvc.AssignSpool(ctx, printerID, spoolID)
						if err != nil {
							return
						}
						break
					}
				}
			}
			result, err = printerSvc.UploadLocalFile(ctx, req, startPrint)
		}, func() {
			if closed {
				return
			}
			uploadBtn.SetSensitive(true)
			chooseBtn.SetSensitive(true)
			showLaunchResult(t, messageLabel, result, err)
			if err == nil && result != nil && !result.Filament.Blocked && !result.RequiresUserConfirmation {
				dialog.Destroy()
				if onSuccess != nil {
					onSuccess()
				}
			}
		})
	})

	dialog.SetVisible(true)
}
