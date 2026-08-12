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
	"github.com/diamondburned/gotk4/pkg/pango"
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

	modelsList := gtk.NewListBox()
	modelsList.SetSelectionMode(gtk.SelectionNone)
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
		clearListBox(modelsList)
		loading := gtk.NewLabel("Loading…")
		loading.AddCSSClass("dim-label")
		modelsList.Append(loading)
		refreshModelsBtn.SetSensitive(false)

		var models []domain.PrinterModelFile
		var err error
		runAsyncUI(func() {
			models, err = printerSvc.RefreshAllModels(ctx)
		}, func() {
			refreshModelsBtn.SetSensitive(true)
			clearListBox(modelsList)
			if err != nil {
				label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				label.SetHAlign(gtk.AlignStart)
				modelsList.Append(label)
				return
			}
			renderPrinterModels(t, modelsList, messageLabel, printerSvc, settingsSvc, ctx, models, printers, spools, onChanged)
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
		clearListBox(modelsList)
		loading := gtk.NewLabel("Loading…")
		loading.AddCSSClass("dim-label")
		modelsList.Append(loading)
		var models []domain.PrinterModelFile
		var err error
		runAsyncUI(func() {
			models, err = printerSvc.ListAllModels(ctx)
		}, func() {
			clearListBox(modelsList)
			if err != nil {
				label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				label.SetHAlign(gtk.AlignStart)
				modelsList.Append(label)
				return
			}
			renderPrinterModels(t, modelsList, messageLabel, printerSvc, settingsSvc, ctx, models, printers, spools, onChanged)
		})
	}
	refreshScreen()
	return box, refreshScreen
}

func renderPrinterModels(t i18n.T, list *gtk.ListBox, messageLabel *gtk.Label, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, ctx context.Context, models []domain.PrinterModelFile, printers []domain.Printer, spools []domain.Spool, onChanged func()) {
	if len(models) == 0 {
		empty := gtk.NewLabel(t.NoPrinterModels)
		empty.AddCSSClass("dim-label")
		list.Append(empty)
		return
	}
	printerNames := printerNamesByID(printers)
	for _, model := range models {
		model := model
		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.SetMarginTop(4)
		row.SetMarginBottom(4)

		if preview := newScaledPreview(model.PreviewImagePath); preview != nil {
			row.Append(preview)
		}

		textBox := gtk.NewBox(gtk.OrientationVertical, 2)
		textBox.SetHExpand(true)
		row.Append(textBox)

		modelName := model.Filename
		if modelName == "" {
			modelName = model.Path
		}
		nameLabel := gtk.NewLabel(modelName)
		nameLabel.SetHAlign(gtk.AlignStart)
		nameLabel.SetHExpand(true)
		nameLabel.SetEllipsize(pango.EllipsizeEnd)
		nameLabel.SetMaxWidthChars(80)
		textBox.Append(nameLabel)

		details := fmt.Sprintf("%s — %.1f g", printerNames[model.PrinterID], model.EstimatedWeightG)
		if model.PrintDuration > 0 {
			details = fmt.Sprintf("%s — %s", details, model.PrintDuration.String())
		}
		detailsLabel := gtk.NewLabel(details)
		detailsLabel.SetHAlign(gtk.AlignStart)
		detailsLabel.AddCSSClass("dim-label")
		detailsLabel.SetEllipsize(pango.EllipsizeEnd)
		detailsLabel.SetMaxWidthChars(80)
		textBox.Append(detailsLabel)

		printBtn := gtk.NewButtonWithLabel(t.PrintModel)
		printBtn.ConnectClicked(func() {
			showPrintLaunchDialog(t, ctx, printerSvc, settingsSvc, printers, spools, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
				req.SourcePrinterID = model.PrinterID
				req.GcodeFilename = model.Path
				return printerSvc.PrintModel(ctx, req)
			}, messageLabel, onChanged)
		})
		row.Append(printBtn)
		list.Append(row)
	}
}

func showPrintLaunchDialog(t i18n.T, ctx context.Context, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, launch func(domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error), messageLabel *gtk.Label, onSuccess func()) {
	dialog := gtk.NewWindow()
	dialog.SetTitle(t.PrintModelDialog)
	dialog.SetDefaultSize(360, 220)
	dialog.SetModal(true)

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
