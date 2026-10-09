package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/screens/shared"

	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func NewModelsScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), *gtk.Button, *gtk.Button) {
	view := layout.Use("models-screen", views...)

	t := i18n.L()

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Models) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	var printers []domain.Printer
	var spools []domain.Spool

	addModelBtn := layout.Get[*gtk.Button](view, "addModelBtn")
	addModelBtn.SetTooltipText(t.UploadModel)
	refreshModelsBtn := layout.Get[*gtk.Button](view, "refreshModelsBtn")
	refreshModelsBtn.SetTooltipText(t.RefreshModels)

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoPrinterModels) })

	_, updateModelsList := newVirtualModelsList(t, ctx, parent, printerSvc, settingsSvc, messageLabel, onChanged, layout.Scope(view, "object_634_slot"))

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
		loaded, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &notArchived})
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

	addModelBtn.ConnectClicked(func() {
		showModelUploadDialog(t, ctx, parent, printerSvc, settingsSvc, printers, spools, messageLabel, func() {
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
	return box, refreshScreen, addModelBtn, refreshModelsBtn
}

func showPrintLaunchDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, defaultPrinterID string, launch func(domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error), messageLabel *gtk.Label, onSuccess func()) {
	view := layout.Load("print-launch-dialog")

	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(t.PrintModelDialog)

	closed := false
	dialog.ConnectDestroy(func() {
		closed = true
	})
	_ = layout.Configure[*gtk.Label](view, "printerLabel", func(w *gtk.Label) { w.SetLabel(t.Printers) })

	printerNames := make([]string, len(printers))
	for i, printer := range printers {
		printerNames[i] = printer.Name
	}
	if len(printerNames) == 0 {
		printerNames = []string{t.NoPrinters}
	}
	printerCombo := layout.Configure[*gtk.DropDown](view, "printerCombo", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(printerNames)) })
	for i, printer := range printers {
		if printer.ID == defaultPrinterID {
			printerCombo.SetSelected(uint(i))
			break
		}
	}
	_ = layout.Configure[*gtk.Label](view, "spoolLabel", func(w *gtk.Label) { w.SetLabel(t.SpoolColon) })

	spoolNames := make([]string, len(spools))
	for i, spool := range spools {
		spoolNames[i] = shared.SpoolDisplayName(spool)
	}
	if len(spoolNames) == 0 {
		spoolNames = []string{t.NoSpoolsAvail}
	}
	spoolCombo := layout.Configure[*gtk.DropDown](view, "spoolCombo", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(spoolNames)) })
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

	cancelBtn := layout.Configure[*gtk.Button](view, "cancelBtn", func(w *gtk.Button) { w.SetLabel(t.Cancel) })
	cancelBtn.ConnectClicked(func() {
		dialog.Destroy()
	})

	printBtn := layout.Configure[*gtk.Button](view, "printBtn", func(w *gtk.Button) { w.SetLabel(t.PrintModel) })
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
