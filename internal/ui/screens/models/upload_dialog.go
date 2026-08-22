package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"path/filepath"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func showModelUploadDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, messageLabel *gtk.Label, onSuccess func()) {
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(t.UploadModel)
	dialog.SetDefaultSize(500, -1)
	closed := false
	dialog.ConnectDestroy(func() { closed = true })
	box := widgets.NewDialogContent(16)

	var localPath string
	fileGroup := adw.NewPreferencesGroup()
	fileRow := adw.NewActionRow()
	fileIcon := gtk.NewImageFromIconName("text-x-generic-symbolic")
	fileRow.AddPrefix(fileIcon)
	fileRow.SetUseMarkup(false)
	fileRow.SetTitle(t.NoFileSelected)
	fileRow.SetSubtitle(t.SelectGCode)
	chooseBtn := gtk.NewButtonWithLabel(t.SelectGCode)
	chooseBtn.SetVAlign(gtk.AlignCenter)
	fileRow.AddSuffix(chooseBtn)
	fileRow.SetActivatableWidget(chooseBtn)
	fileGroup.Add(fileRow)
	box.Append(fileGroup)

	printerNames := make([]string, len(printers))
	for i, printer := range printers {
		printerNames[i] = printer.Name
	}
	if len(printerNames) == 0 {
		printerNames = []string{t.NoPrinters}
	}
	optionsGroup := adw.NewPreferencesGroup()
	printerRow := adw.NewActionRow()
	printerIcon := gtk.NewImageFromIconName("printer-symbolic")
	printerRow.AddPrefix(printerIcon)
	printerRow.SetTitle(t.Printers)
	printerCombo := gtk.NewDropDown(gtk.NewStringList(printerNames), nil)
	printerCombo.SetVAlign(gtk.AlignCenter)
	printerRow.AddSuffix(printerCombo)
	printerRow.SetActivatableWidget(printerCombo)
	optionsGroup.Add(printerRow)

	startCheck := gtk.NewCheckButton()
	startCheck.SetVAlign(gtk.AlignCenter)
	startRow := adw.NewActionRow()
	startIcon := gtk.NewImageFromIconName("media-playback-start-symbolic")
	startRow.AddPrefix(startIcon)
	startRow.SetTitle(t.StartAfterUpload)
	startRow.AddSuffix(startCheck)
	startRow.SetActivatableWidget(startCheck)
	optionsGroup.Add(startRow)

	spoolNames := make([]string, len(spools))
	for i, spool := range spools {
		spoolNames[i] = shared.SpoolDisplayName(spool)
	}
	if len(spoolNames) == 0 {
		spoolNames = []string{t.NoSpoolsAvail}
	}
	spoolRow := adw.NewActionRow()
	spoolIcon := gtk.NewImageFromIconName("media-record-symbolic")
	spoolRow.AddPrefix(spoolIcon)
	spoolRow.SetTitle(t.SpoolColon)
	spoolCombo := gtk.NewDropDown(gtk.NewStringList(spoolNames), nil)
	spoolCombo.SetVAlign(gtk.AlignCenter)
	spoolRow.AddSuffix(spoolCombo)
	spoolRow.SetActivatableWidget(spoolCombo)
	spoolRow.SetVisible(false)
	optionsGroup.Add(spoolRow)
	box.Append(optionsGroup)

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

	actions := widgets.NewActions(8, gtk.AlignEnd)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(dialog.Destroy)
	actions.Append(cancelBtn)
	uploadBtn := gtk.NewButtonWithLabel(t.UploadModel)
	uploadBtn.AddCSSClass("suggested-action")
	uploadBtn.SetSensitive(false)
	actions.Append(uploadBtn)
	box.Append(actions)

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

	dialog.SetChild(box)
	dialog.SetVisible(true)
}
