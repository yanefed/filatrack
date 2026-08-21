package screens

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"context"
	"fmt"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

type modelListState struct {
	models       []domain.PrinterModelFile
	printers     []domain.Printer
	spools       []domain.Spool
	printerNames map[string]string
}

type modelRowWidgets struct {
	picture *gtk.Picture
	name    *gtk.Label
	details *gtk.Label
}

func newVirtualModelsList(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, messageLabel *gtk.Label, onChanged func()) (*gtk.ListView, func([]domain.PrinterModelFile, []domain.Printer, []domain.Spool)) {
	state := &modelListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*modelRowWidgets)

	factory.ConnectSetup(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		item.SetActivatable(false)
		item.SetFocusable(false)

		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.SetMarginTop(4)
		row.SetMarginBottom(4)

		picture := gtk.NewPicture()
		picture.SetSizeRequest(previewWidth, previewHeight)
		picture.SetContentFit(gtk.ContentFitContain)
		picture.SetVisible(false)
		row.Append(picture)

		textBox := gtk.NewBox(gtk.OrientationVertical, 2)
		textBox.SetHExpand(true)
		name := gtk.NewLabel("")
		name.SetHAlign(gtk.AlignStart)
		name.SetHExpand(true)
		name.SetEllipsize(pango.EllipsizeEnd)
		name.SetMaxWidthChars(80)
		textBox.Append(name)

		details := gtk.NewLabel("")
		details.SetHAlign(gtk.AlignStart)
		details.AddCSSClass("dim-label")
		details.SetEllipsize(pango.EllipsizeEnd)
		details.SetMaxWidthChars(80)
		textBox.Append(details)
		row.Append(textBox)

		printBtn := gtk.NewButtonWithLabel(t.PrintModel)
		printBtn.ConnectClicked(func() {
			position := item.Position()
			if int(position) >= len(state.models) {
				return
			}
			selected := state.models[position]
			showPrintLaunchDialog(t, ctx, parent, printerSvc, settingsSvc, state.printers, state.spools, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
				req.SourcePrinterID = selected.PrinterID
				req.GcodeFilename = selected.Path
				return printerSvc.PrintModel(ctx, req)
			}, messageLabel, onChanged)
		})
		row.Append(printBtn)
		item.SetChild(row)
		rows[item.Native()] = &modelRowWidgets{picture: picture, name: name, details: details}
	})

	factory.ConnectBind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		widgets := rows[item.Native()]
		position := item.Position()
		if widgets == nil || int(position) >= len(state.models) {
			return
		}
		entry := state.models[position]
		name := entry.Filename
		if name == "" {
			name = entry.Path
		}
		widgets.name.SetLabel(name)
		details := fmt.Sprintf("%s — %.1f g", state.printerNames[entry.PrinterID], entry.EstimatedWeightG)
		if entry.PrintDuration > 0 {
			details = fmt.Sprintf("%s — %s", details, entry.PrintDuration.String())
		}
		widgets.details.SetLabel(details)
		texture := loadScaledPreviewTexture(entry.PreviewImagePath)
		if texture != nil {
			widgets.picture.SetPaintable(texture)
			widgets.picture.SetVisible(true)
		} else {
			widgets.picture.SetPaintable(nil)
			widgets.picture.SetVisible(false)
		}
	})

	factory.ConnectUnbind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		if widgets := rows[item.Native()]; widgets != nil {
			// Textures are the largest row resource. Drop them as soon as a row
			// leaves the virtualized viewport.
			widgets.picture.SetPaintable(nil)
			widgets.picture.SetVisible(false)
		}
	})
	factory.ConnectTeardown(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		delete(rows, item.Native())
	})

	view := gtk.NewListView(selection, &factory.ListItemFactory)
	view.SetVExpand(true)
	update := func(models []domain.PrinterModelFile, printers []domain.Printer, spools []domain.Spool) {
		state.models = models
		state.printers = printers
		state.spools = spools
		state.printerNames = printerNamesByID(printers)
		model.Splice(0, model.NItems(), modelKeys(models))
	}
	return view, update
}

func modelKeys(models []domain.PrinterModelFile) []string {
	keys := make([]string, len(models))
	for i := range models {
		keys[i] = models[i].PrinterID + "\x00" + models[i].Path
	}
	return keys
}

type printListState struct {
	jobs   []domain.PrintJob
	spools []domain.Spool
}

type printRowWidgets struct {
	picture   *gtk.Picture
	header    *gtk.Label
	details   *gtk.Label
	assignRow *gtk.Box
	assignBtn *gtk.Button
}

func newVirtualPrintsList(t i18n.T, ctx context.Context, printSvc *service.PrintService, messageLabel *gtk.Label) (*gtk.ListView, func([]domain.PrintJob, []domain.Spool)) {
	state := &printListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*printRowWidgets)
	var activeJobID string
	var activeButton *gtk.Button
	var activeRow *gtk.Box
	chooserModel := gtk.NewStringList(nil)
	chooser := gtk.NewDropDown(chooserModel, nil)
	changingSelection := false
	finishChoosing := func() {
		if activeRow != nil && chooser.Parent() != nil {
			activeRow.Remove(chooser)
			if activeButton != nil {
				activeRow.Append(activeButton)
			}
		}
		activeJobID = ""
		activeButton = nil
		activeRow = nil
	}
	chooser.NotifyProperty("selected", func() {
		if changingSelection || activeJobID == "" {
			return
		}
		idx := chooser.Selected()
		if idx == gtk.InvalidListPosition || int(idx) > len(state.spools) {
			return
		}
		spoolID := ""
		if idx > 0 {
			spoolID = state.spools[idx-1].ID
		}
		jobIndex := -1
		for i := range state.jobs {
			if state.jobs[i].ID == activeJobID {
				jobIndex = i
				break
			}
		}
		if jobIndex < 0 {
			finishChoosing()
			return
		}

		job := &state.jobs[jobIndex]
		if spoolID == job.SpoolID {
			finishChoosing()
			return
		}
		if err := printSvc.ChangeSpool(ctx, job.ID, spoolID); err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
			messageLabel.SetVisible(true)
			return
		}
		job.SpoolID = spoolID
		if activeButton != nil {
			activeButton.SetLabel(printSpoolButtonLabel(t, spoolID, state.spools))
		}
		messageLabel.SetVisible(false)
		finishChoosing()
	})
	focus := gtk.NewEventControllerFocus()
	focus.ConnectLeave(func() {
		coreglib.IdleAdd(func() {
			if activeRow != nil && !focus.ContainsFocus() {
				finishChoosing()
			}
		})
	})
	chooser.AddController(focus)
	setChooserSpools := func(spools []domain.Spool) {
		names := spoolChoiceNames(t, spools)
		chooserModel.Splice(0, chooserModel.NItems(), names)
	}

	factory.ConnectSetup(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		item.SetActivatable(false)
		item.SetFocusable(false)

		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.SetMarginTop(6)
		row.SetMarginBottom(6)
		picture := gtk.NewPicture()
		picture.SetSizeRequest(previewWidth, previewHeight)
		picture.SetContentFit(gtk.ContentFitContain)
		picture.SetVisible(false)
		row.Append(picture)

		content := gtk.NewBox(gtk.OrientationVertical, 4)
		content.SetHExpand(true)
		header := gtk.NewLabel("")
		header.SetHAlign(gtk.AlignStart)
		header.SetHExpand(true)
		header.SetEllipsize(pango.EllipsizeEnd)
		header.SetMaxWidthChars(80)
		content.Append(header)
		details := gtk.NewLabel("")
		details.SetHAlign(gtk.AlignStart)
		details.AddCSSClass("dim-label")
		content.Append(details)

		assignRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
		assignBtn := gtk.NewButtonWithLabel(t.SpoolColon)
		assignRow.Append(assignBtn)
		content.Append(assignRow)

		row.Append(content)
		item.SetChild(row)

		widgets := &printRowWidgets{picture: picture, header: header, details: details, assignRow: assignRow, assignBtn: assignBtn}
		rows[item.Native()] = widgets

		assignBtn.ConnectClicked(func() {
			widgets := rows[item.Native()]
			if widgets == nil {
				return
			}
			position := item.Position()
			if position == gtk.InvalidListPosition || int(position) >= len(state.jobs) {
				return
			}

			if activeRow != nil {
				finishChoosing()
			}
			activeJobID = state.jobs[position].ID
			activeButton = assignBtn
			activeRow = widgets.assignRow
			changingSelection = true
			chooser.SetSelected(selectedSpoolIndex(state.jobs[position].SpoolID, state.spools))
			changingSelection = false
			widgets.assignRow.Remove(assignBtn)
			widgets.assignRow.Append(chooser)
			chooser.GrabFocus()
			coreglib.IdleAdd(func() {
				if activeRow == widgets.assignRow {
					chooser.Activate()
					limitDropDownHeight(chooser, 320)
				}
			})
		})
	})

	factory.ConnectBind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		widgets := rows[item.Native()]
		position := item.Position()
		if widgets == nil || int(position) >= len(state.jobs) {
			return
		}
		job := state.jobs[position]
		statusIcon := "✅"
		switch job.Status {
		case "failed":
			statusIcon = "❌"
		case "cancelled":
			statusIcon = "🚫"
		}
		title := job.ModelName
		if title == "" {
			title = job.GcodeFilename
		}
		widgets.header.SetLabel(fmt.Sprintf("%s %s", statusIcon, title))
		widgets.details.SetLabel(printJobUsageText(t, job))
		texture := loadScaledPreviewTexture(job.PreviewImagePath)
		if texture != nil {
			widgets.picture.SetPaintable(texture)
			widgets.picture.SetVisible(true)
		} else {
			widgets.picture.SetPaintable(nil)
			widgets.picture.SetVisible(false)
		}
		widgets.assignRow.SetVisible(len(state.spools) > 0)
		if activeButton == widgets.assignBtn {
			finishChoosing()
		}
		widgets.assignBtn.SetLabel(printSpoolButtonLabel(t, job.SpoolID, state.spools))
	})

	factory.ConnectUnbind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		if widgets := rows[item.Native()]; widgets != nil {
			widgets.picture.SetPaintable(nil)
			widgets.picture.SetVisible(false)
		}
	})
	factory.ConnectTeardown(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		delete(rows, item.Native())
	})

	view := gtk.NewListView(selection, &factory.ListItemFactory)
	view.SetVExpand(true)
	update := func(jobs []domain.PrintJob, spools []domain.Spool) {
		finishChoosing()
		state.jobs = jobs
		state.spools = spools
		setChooserSpools(spools)
		keys := make([]string, len(jobs))
		for i := range jobs {
			keys[i] = jobs[i].ID
		}
		model.Splice(0, model.NItems(), keys)
	}
	return view, update
}

func printSpoolButtonLabel(t i18n.T, spoolID string, spools []domain.Spool) string {
	for _, spool := range spools {
		if spool.ID == spoolID {
			return fmt.Sprintf(" %s", spoolDisplayName(spool))
		}
	}
	return fmt.Sprintf("%s %s", t.SpoolColon, t.UnassignedSpool)
}

func limitDropDownHeight(root gtk.Widgetter, maxHeight int) {
	if scrolled, ok := root.(*gtk.ScrolledWindow); ok {
		scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
		scrolled.SetPropagateNaturalHeight(true)
		scrolled.SetMaxContentHeight(maxHeight)
	}
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		limitDropDownHeight(child, maxHeight)
	}
}
