package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func NewModelsScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
	t := i18n.L()

	box := widgets.NewScreen(t.Models)

	messageLabel := widgets.NewMessageLabel()
	box.Append(messageLabel)

	var printers []domain.Printer
	var spools []domain.Spool

	box.Append(widgets.NewSectionTitle(t.UploadAndPrint))
	pathRow, upload := newModelUpload(t, parent)
	box.Append(pathRow)
	box.Append(upload.button)

	box.Append(widgets.NewSectionTitle(t.Models))

	refreshModelsBtn := gtk.NewButtonWithLabel(t.RefreshModels)
	box.Append(refreshModelsBtn)

	emptyLabel := gtk.NewLabel(t.NoPrinterModels)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)
	modelsList, updateModelsList := newVirtualModelsList(t, ctx, parent, printerSvc, settingsSvc, messageLabel, onChanged)
	box.Append(widgets.NewScrolled(modelsList))

	refreshPrinterChoices := func() {
		loaded, err := printerSvc.List(ctx)
		if err != nil {
			printers = nil
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			messageLabel.SetVisible(true)
			return
		}
		printers = loaded
	}

	refreshSpoolChoices := func() {
		notArchived := false
		loaded, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			spools = nil
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			messageLabel.SetVisible(true)
			return
		}
		spools = loaded
	}

	refreshModels := func() {
		updateModelsList(nil, printers, spools)
		emptyLabel.SetLabel("Loading…")
		emptyLabel.SetVisible(true)
		refreshModelsBtn.SetSensitive(false)

		var models []domain.PrinterModelFile
		var err error
		shared.RunAsyncUI(func() {
			models, err = printerSvc.RefreshAllModels(ctx)
		}, func() {
			refreshModelsBtn.SetSensitive(true)
			if err != nil {
				emptyLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				emptyLabel.SetVisible(true)
				return
			}
			emptyLabel.SetLabel(t.NoPrinterModels)
			emptyLabel.SetVisible(len(models) == 0)
			updateModelsList(models, printers, spools)
		})
	}

	upload.button.ConnectClicked(func() {
		localPath := strings.TrimSpace(upload.Path())
		if localPath == "" {
			return
		}
		showPrintLaunchDialog(t, ctx, parent, printerSvc, settingsSvc, printers, spools, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
			req.LocalPath = localPath
			return printerSvc.UploadAndStartLocalFile(ctx, req)
		}, messageLabel, func() {
			upload.Reset()
			refreshModels()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	refreshModelsBtn.ConnectClicked(refreshModels)

	refreshScreen := func() {
		refreshPrinterChoices()
		refreshSpoolChoices()
		updateModelsList(nil, printers, spools)
		emptyLabel.SetLabel("Loading…")
		emptyLabel.SetVisible(true)
		var models []domain.PrinterModelFile
		var err error
		shared.RunAsyncUI(func() {
			models, err = printerSvc.ListAllModels(ctx)
		}, func() {
			if err != nil {
				emptyLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				emptyLabel.SetVisible(true)
				return
			}
			emptyLabel.SetLabel(t.NoPrinterModels)
			emptyLabel.SetVisible(len(models) == 0)
			updateModelsList(models, printers, spools)
		})
	}
	refreshScreen()
	return box, refreshScreen
}

func showPrintLaunchDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, launch func(domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error), messageLabel *gtk.Label, onSuccess func()) {
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(t.PrintModelDialog)
	dialog.SetDefaultSize(360, 220)
	closed := false
	dialog.ConnectDestroy(func() {
		closed = true
	})

	box := widgets.NewDialogContent(10)

	printerLabel := gtk.NewLabel(t.Printers)
	printerLabel.SetHAlign(gtk.AlignStart)
	box.Append(printerLabel)

	printerNames := make([]string, len(printers))
	for i, printer := range printers {
		printerNames[i] = printer.Name
	}
	if len(printerNames) == 0 {
		printerNames = []string{t.NoPrinters}
	}
	printerCombo := gtk.NewDropDown(gtk.NewStringList(printerNames), nil)
	box.Append(printerCombo)

	spoolLabel := gtk.NewLabel(t.SpoolColon)
	spoolLabel.SetHAlign(gtk.AlignStart)
	box.Append(spoolLabel)

	spoolNames := make([]string, len(spools))
	for i, spool := range spools {
		spoolNames[i] = shared.SpoolDisplayName(spool)
	}
	if len(spoolNames) == 0 {
		spoolNames = []string{t.NoSpoolsAvail}
	}
	spoolCombo := gtk.NewDropDown(gtk.NewStringList(spoolNames), nil)
	setSpoolFromSelectedPrinter := func() {
		printerIndex := printerCombo.Selected()
		if int(printerIndex) >= len(printers) {
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
	setSpoolFromSelectedPrinter()
	printerCombo.NotifyProperty("selected", func() {
		setSpoolFromSelectedPrinter()
	})
	box.Append(spoolCombo)

	actions := widgets.NewActions(8, gtk.AlignFill)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(func() {
		dialog.Destroy()
	})
	actions.Append(cancelBtn)

	printBtn := gtk.NewButtonWithLabel(t.PrintModel)
	printBtn.ConnectClicked(func() {
		targetID := selectedPrinterID(printerCombo, printers)
		spoolID := selectedDirectSpool(spoolCombo, spools)
		if targetID == "" {
			return
		}
		req := domain.PrintLaunchRequest{
			PrinterID:        targetID,
			SpoolID:          spoolID,
			AllowLowFilament: settingsSvc.AllowLowFilamentWarning(ctx),
		}
		printBtn.SetSensitive(false)
		messageLabel.SetLabel("Loading…")
		messageLabel.SetVisible(true)
		var result *domain.PrintLaunchResult
		var err error
		shared.RunAsyncUI(func() {
			if spoolID != "" {
				for i := range printers {
					if printers[i].ID == targetID && printers[i].ActiveSpoolID != spoolID {
						if assignErr := printerSvc.AssignSpool(ctx, targetID, spoolID); assignErr != nil {
							err = assignErr
							return
						}
						break
					}
				}
			}
			result, err = launch(req)
		}, func() {
			if closed {
				return
			}
			printBtn.SetSensitive(true)
			showLaunchResult(t, messageLabel, result, err)
			if err == nil && result != nil && !result.Filament.Blocked && !result.RequiresUserConfirmation {
				dialog.Destroy()
				if onSuccess != nil {
					onSuccess()
				}
			}
		})
	})
	actions.Append(printBtn)
	box.Append(actions)

	dialog.SetChild(box)
	dialog.SetVisible(true)
}

func selectedPrinterID(dropdown *gtk.DropDown, printers []domain.Printer) string {
	idx := dropdown.Selected()
	if int(idx) >= len(printers) {
		return ""
	}
	return printers[idx].ID
}

func printerNamesByID(printers []domain.Printer) map[string]string {
	names := make(map[string]string, len(printers))
	for _, printer := range printers {
		names[printer.ID] = printer.Name
	}
	return names
}
