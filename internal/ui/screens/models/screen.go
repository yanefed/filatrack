package models

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

func NewModelsScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), *gtk.Button, *gtk.Button, *gtk.MenuButton) {
	view := layout.Use("models-screen", views...)

	t := i18n.L()

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Models) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")
	emptyLabel := layout.Get[*gtk.Label](view, "emptyLabel")
	emptyLabel.SetLabel(t.NoPrinterModels)
	addModelBtn := layout.Get[*gtk.Button](view, "addModelBtn")
	addModelBtn.SetTooltipText(t.UploadModel)
	refreshModelsBtn := layout.Get[*gtk.Button](view, "refreshModelsBtn")
	refreshModelsBtn.SetTooltipText(t.RefreshModels)
	optionsBtn := layout.Get[*gtk.MenuButton](view, "modelOptionsBtn")
	optionsBtn.SetTooltipText(t.ModelRefreshOptions)
	forceBtn := layout.Get[*gtk.Button](view, "refreshAllMetadataBtn")
	forceBtn.SetLabel(t.RefreshAllModelMetadata)
	popover := layout.Get[*gtk.Popover](view, "modelOptionsPopover")
	var printers []domain.Printer
	var spools []domain.Spool
	_, updateModelsList := newVirtualModelsList(t, ctx, parent, printerSvc, settingsSvc, messageLabel, onChanged, layout.Scope(view, "object_634_slot"))
	type result struct {
		models   []domain.PrinterModelFile
		printers []domain.Printer
		spools   []domain.Spool
		report   service.ModelRefreshResult
		err      error
		remote   bool
	}
	loader := shared.NewLoader[result](ctx, box)
	refreshing, reloadAfterRefresh := false, false
	var loadCached func()
	request := func(remote bool, mode service.ModelRefreshMode) {
		if remote {
			refreshing = true
			refreshModelsBtn.SetSensitive(false)
			optionsBtn.SetSensitive(false)
			messageLabel.SetLabel(t.RefreshingModels)
			messageLabel.SetVisible(true)
		}
		loader.Request(func(ctx context.Context) result {
			data := result{remote: remote}
			data.printers, data.err = printerSvc.List(ctx)
			if data.err != nil {
				return data
			}
			active := false
			data.spools, data.err = spoolSvc.List(ctx, domain.SpoolFilter{Archived: &active})
			if data.err != nil {
				return data
			}
			if remote {
				data.report, data.err = printerSvc.RefreshModelCatalog(ctx, mode)
				data.models = data.report.Models
			} else {
				data.models, data.err = printerSvc.ListAllModels(ctx)
			}
			return data
		}, func(data result) {
			if data.remote {
				refreshing = false
				refreshModelsBtn.SetSensitive(true)
				optionsBtn.SetSensitive(true)
			}
			if data.err != nil {
				messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, data.err))
				messageLabel.SetVisible(true)
			} else {
				printers = data.printers
				spools = data.spools
				emptyLabel.SetLabel(t.NoPrinterModels)
				emptyLabel.SetVisible(len(data.models) == 0)
				updateModelsList(data.models, printers, spools)
				if data.remote {
					messageLabel.SetVisible(len(data.report.Failures) > 0)
					if len(data.report.Failures) > 0 {
						messageLabel.SetLabel(fmt.Sprintf(t.PartialModelRefresh, len(data.report.Failures)) + "\n" + data.report.Failures[0].Error())
					}
				}
			}
			if reloadAfterRefresh {
				reloadAfterRefresh = false
				loadCached()
			}
		})
	}
	loadCached = func() { request(false, service.ModelRefreshChanges) }
	refreshModels := func() {
		if !refreshing {
			request(true, service.ModelRefreshChanges)
		}
	}
	refreshModelsBtn.ConnectClicked(refreshModels)
	forceBtn.ConnectClicked(func() {
		popover.Popdown()
		if !refreshing {
			request(true, service.ModelRefreshFull)
		}
	})
	addModelBtn.ConnectClicked(func() {
		showModelUploadDialog(t, ctx, parent, printerSvc, settingsSvc, printers, spools, messageLabel, func() {
			refreshModels()
			if onChanged != nil {
				onChanged()
			}
		})
	})
	refreshScreen := func() {
		if refreshing {
			reloadAfterRefresh = true
			return
		}
		loadCached()
	}
	refreshScreen()
	return box, refreshScreen, addModelBtn, refreshModelsBtn, optionsBtn
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
			PrinterID: targetID,
			SpoolID:   spoolID,
		}
		printBtn.SetSensitive(false)
		messageLabel.SetLabel("Loading…")
		messageLabel.SetVisible(true)
		var result *domain.PrintLaunchResult
		var err error
		shared.RunAsyncUI(ctx, func() {
			req.AllowLowFilament = settingsSvc.AllowLowFilamentWarning(ctx)
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
