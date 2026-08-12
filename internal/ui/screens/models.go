package screens

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func NewModelsScreen(ctx context.Context, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
	t := i18n.L()

	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	title := gtk.NewLabel(t.Models)
	title.AddCSSClass("title-1")
	box.Append(title)

	messageLabel := gtk.NewLabel("")
	messageLabel.SetHAlign(gtk.AlignStart)
	messageLabel.AddCSSClass("dim-label")
	messageLabel.SetWrap(true)
	messageLabel.SetVisible(false)
	box.Append(messageLabel)

	var printers []domain.Printer
	var spools []domain.Spool

	uploadTitle := gtk.NewLabel(t.UploadAndPrint)
	uploadTitle.AddCSSClass("title-3")
	uploadTitle.SetHAlign(gtk.AlignStart)
	box.Append(uploadTitle)

	pathEntry := gtk.NewEntry()
	pathEntry.SetPlaceholderText(t.LocalGCodePath)
	pathRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	pathEntry.SetHExpand(true)
	pathRow.Append(pathEntry)
	chooseFileBtn := gtk.NewButtonWithLabel(t.SelectGCode)
	chooseFileBtn.ConnectClicked(func() {
		showGCodeFileChooser(pathEntry)
	})
	pathRow.Append(chooseFileBtn)
	box.Append(pathRow)

	uploadBtn := gtk.NewButtonWithLabel(t.UploadAndPrint)
	box.Append(uploadBtn)

	modelsTitle := gtk.NewLabel(t.Models)
	modelsTitle.AddCSSClass("title-3")
	modelsTitle.SetHAlign(gtk.AlignStart)
	box.Append(modelsTitle)

	refreshModelsBtn := gtk.NewButtonWithLabel(t.RefreshModels)
	box.Append(refreshModelsBtn)

	emptyLabel := gtk.NewLabel(t.NoPrinterModels)
	emptyLabel.AddCSSClass("dim-label")
	emptyLabel.SetVisible(false)
	box.Append(emptyLabel)
	modelsList, updateModelsList := newVirtualModelsList(t, ctx, printerSvc, settingsSvc, messageLabel, onChanged)
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(modelsList)
	box.Append(scrolled)

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
		runAsyncUI(func() {
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

	uploadBtn.ConnectClicked(func() {
		localPath := strings.TrimSpace(pathEntry.Text())
		if localPath == "" {
			return
		}
		showPrintLaunchDialog(t, ctx, printerSvc, settingsSvc, printers, spools, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
			req.LocalPath = localPath
			return printerSvc.UploadAndStartLocalFile(ctx, req)
		}, messageLabel, func() {
			pathEntry.SetText("")
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
		runAsyncUI(func() {
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

func showPrintLaunchDialog(t i18n.T, ctx context.Context, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, launch func(domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error), messageLabel *gtk.Label, onSuccess func()) {
	dialog := gtk.NewWindow()
	dialog.SetTitle(t.PrintModelDialog)
	dialog.SetDefaultSize(360, 220)
	dialog.SetModal(true)
	closed := false
	dialog.ConnectDestroy(func() {
		closed = true
	})

	box := gtk.NewBox(gtk.OrientationVertical, 10)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)

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
		spoolNames[i] = spoolDisplayName(spool)
	}
	if len(spoolNames) == 0 {
		spoolNames = []string{t.NoSpoolsAvail}
	}
	spoolCombo := gtk.NewDropDown(gtk.NewStringList(spoolNames), nil)
	setSpoolFromSelectedPrinter := func() {
		idx := printerCombo.Selected()
		if int(idx) >= len(printers) {
			return
		}
		spoolCombo.SetSelected(selectedSpoolIndex(printers[idx].ActiveSpoolID, spools))
	}
	setSpoolFromSelectedPrinter()
	printerCombo.NotifyProperty("selected", func() {
		setSpoolFromSelectedPrinter()
	})
	box.Append(spoolCombo)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
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
		runAsyncUI(func() {
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
