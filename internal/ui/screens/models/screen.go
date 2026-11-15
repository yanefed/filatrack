package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

func NewModelsScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), *gtk.Button, *gtk.Button, *gtk.MenuButton) {
	view := layout.Use("models-screen", views...)

	t := i18n.L()

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Models) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")
	emptyLabel := layout.Get[*gtk.Label](view, "emptyLabel")
	emptyLabel.SetLabel(t.NoPrinterModels)
	emptyLabel.SetVisible(false)
	messageLabel.SetText(t.Loading)
	messageLabel.SetVisible(true)
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

	var catalog []domain.PrinterModelFile
	var choices []domain.Printer
	filterUpdating := false
	filterModel := gtk.NewStringList([]string{t.AllPrinters})
	filter := gtk.NewDropDown(filterModel, nil)
	box.InsertChildAfter(filter, layout.Get[*gtk.Label](view, "box_title"))
	var search *gtk.SearchEntry
	var emptyAction *gtk.Button
	render := func() {
		query := strings.ToLower(strings.TrimSpace(search.Text()))
		id := ""
		if i := filter.Selected(); i > 0 && int(i) <= len(choices) {
			id = choices[i-1].ID
		}
		visible := make([]domain.PrinterModelFile, 0, len(catalog))
		for _, m := range catalog {
			if id != "" && m.PrinterID != id {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(m.Filename+" "+m.Path), query) {
				continue
			}
			visible = append(visible, m)
		}
		emptyLabel.SetLabel(t.NoPrinterModels)
		if query != "" || id != "" {
			emptyLabel.SetLabel(t.NoSearchResults)
		}
		emptyLabel.SetVisible(len(visible) == 0)
		if emptyAction != nil {
			title := t.UploadModel
			if len(printers) == 0 {
				title = t.AddPrinter
			}
			if query != "" || id != "" {
				title = t.ResetFilters
			}
			emptyAction.SetLabel(title)
		}
		updateModelsList(visible, printers, spools)
	}
	search = shared.SearchBox(box, render)
	filter.NotifyProperty("selected", func() {
		if search != nil && !filterUpdating {
			render()
		}
	})
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
	feedback := shared.NewReadFeedback(box, messageLabel, func() { loadCached() })
	request := func(remote bool, mode service.ModelRefreshMode) {
		feedback.Begin()
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
				feedback.Failure(data.err)
			} else {
				feedback.Success()
				printers = data.printers
				spools = data.spools
				emptyLabel.SetLabel(t.NoPrinterModels)
				emptyLabel.SetVisible(len(data.models) == 0)
				catalog = data.models
				selected := ""
				if i := filter.Selected(); i > 0 && int(i) <= len(choices) {
					selected = choices[i-1].ID
				}
				names := []string{t.AllPrinters}
				for _, p := range printers {
					names = append(names, p.Name)
				}
				choices = printers
				filterUpdating = true
				shared.SyncStrings(filterModel, names)
				index := uint(0)
				for i, p := range choices {
					if p.ID == selected {
						index = uint(i + 1)
					}
				}
				filter.SetSelected(index)
				filterUpdating = false
				render()
				messageLabel.SetVisible(false)
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
	emptyAction = shared.EmptyAction(ctx, box, emptyLabel, t.UploadModel, func() {
		if search.Text() != "" || filter.Selected() != 0 {
			search.SetText("")
			filter.SetSelected(0)
			return
		}
		if len(printers) == 0 {
			shared.OpenPage(box, "printers")
			return
		}
		if shared.ReadOnly(ctx) {
			messageLabel.SetText(t.OfflineAction)
			messageLabel.SetVisible(true)
			return
		}
		addModelBtn.Object.Emit("clicked")
	})
	refreshScreen()
	return box, refreshScreen, addModelBtn, refreshModelsBtn, optionsBtn
}

func showPrintLaunchDialog(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, printers []domain.Printer, spools []domain.Spool, defaultPrinterID, filename string, launch func(domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error), messageLabel *gtk.Label, onSuccess func()) {
	view := layout.Load("print-launch-dialog")

	dialog := layout.Configure[*gtk.Window](view, "dialog", func(w *gtk.Window) {
		if parent != nil {
			w.SetTransientFor(parent)
			w.SetDestroyWithParent(true)
		}
	})
	dialog.SetTitle(t.PrintModelDialog + " — " + filename)

	closed := false
	dialog.ConnectDestroy(func() {
		closed = true
	})
	_ = layout.Configure[*gtk.Label](view, "printerLabel", func(w *gtk.Label) { w.SetLabel(t.Printers) })

	printerNames := make([]string, len(printers))
	for i, printer := range printers {
		printerNames[i] = widgets.PrinterInfoText(t, printer)
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
	content := layout.Get[*gtk.Box](view, "box")
	modelLabel := gtk.NewLabel(filename)
	modelLabel.SetWrap(true)
	modelLabel.SetXAlign(0)
	content.Prepend(modelLabel)
	shared.AccessibleName(printerCombo, t.Printers)
	shared.AccessibleName(spoolCombo, t.Spools)
	status := shared.InlineMessage(content)
	content.ReorderChildAfter(status, spoolCombo)
	busy, confirmed := false, false
	var preview *domain.PrintLaunchResult
	currentRequest := func() domain.PrintLaunchRequest {
		return domain.PrintLaunchRequest{PrinterID: selectedPrinterID(printerCombo, printers), SpoolID: selectedDirectSpool(spoolCombo, spools), SourcePrinterID: defaultPrinterID, GcodeFilename: filename}
	}
	previewLoader := shared.NewLoader[launchPreview](ctx, dialog)
	updateSelection := func() {
		confirmed = false
		preview = nil
		printBtn.SetLabel(t.PrintModel)
		req := currentRequest()
		printBtn.SetSensitive(false)
		if req.PrinterID == "" || req.SpoolID == "" {
			status.SetText(t.NoSpoolsAvail)
			status.SetVisible(true)
			return
		}
		status.SetText(t.Loading)
		status.SetVisible(true)
		previewLoader.Request(func(c context.Context) launchPreview {
			req.AllowLowFilament = settingsSvc.AllowLowFilamentWarning(c)
			result, err := printerSvc.PreviewLaunch(c, req)
			return launchPreview{result, err}
		}, func(data launchPreview) {
			preview = data.result
			showLaunchResult(t, status, data.result, data.err)
			printBtn.SetSensitive(data.err == nil && preview != nil && !preview.Filament.Blocked)
		})
	}
	printerCombo.NotifyProperty("selected", updateSelection)
	spoolCombo.NotifyProperty("selected", updateSelection)
	printBtn.ConnectClicked(func() {
		if busy || preview == nil {
			return
		}
		if preview.RequiresUserConfirmation && !confirmed {
			shared.Confirm(dialog, t.LaunchWarning, filamentText(t, preview.Filament), t.PrintAnyway, func() { confirmed = true; printBtn.SetLabel(t.PrintAnyway); printBtn.Object.Emit("clicked") })
			return
		}
		req := currentRequest()
		req.AllowLowFilament = confirmed
		busy = true
		printBtn.SetSensitive(false)
		printBtn.SetLabel(t.StartingPrint)
		printerCombo.SetSensitive(false)
		spoolCombo.SetSensitive(false)
		cancelBtn.SetSensitive(false)
		var result *domain.PrintLaunchResult
		var err error
		shared.RunAsyncUI(ctx, func() {
			req.AllowLowFilament = req.AllowLowFilament || settingsSvc.AllowLowFilamentWarning(ctx)
			result, err = launch(req)
		}, func() {
			if closed {
				return
			}
			busy = false
			printerCombo.SetSensitive(true)
			spoolCombo.SetSensitive(true)
			cancelBtn.SetSensitive(true)
			printBtn.SetLabel(t.PrintModel)
			printBtn.SetSensitive(true)
			showLaunchResult(t, status, result, err)
			if err != nil && result != nil && result.LaunchAttempted {
				printBtn.SetSensitive(false)
			}
			if err == nil && result != nil {
				preview = result
				if result.Started || result.Queued || result.Uploaded {
					showLaunchResult(t, messageLabel, result, nil)
					shared.Notify(messageLabel, launchSuccessText(t, result), t.OpenPrinter, func() { shared.OpenPage(messageLabel, "printers") })
					dialog.Destroy()
					if onSuccess != nil {
						onSuccess()
					}
				}
			}
		})
	})
	dialog.ConnectCloseRequest(func() bool { return busy })
	updateSelection()

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
