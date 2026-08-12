package screens

import (
	"context"
	"fmt"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"

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

func newVirtualModelsList(t i18n.T, ctx context.Context, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, messageLabel *gtk.Label, onChanged func()) (*gtk.ListView, func([]domain.PrinterModelFile, []domain.Printer, []domain.Spool)) {
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
			showPrintLaunchDialog(t, ctx, printerSvc, settingsSvc, state.printers, state.spools, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
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

func newVirtualPrintsList(t i18n.T, ctx context.Context, printSvc *service.PrintService, messageLabel *gtk.Label, onChanged func()) (*gtk.ListView, func([]domain.PrintJob, []domain.Spool)) {
	state := &printListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*printRowWidgets)

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
			position := item.Position()
			if int(position) >= len(state.jobs) {
				return
			}
			job := &state.jobs[position]
			showPrintSpoolDialog(t, ctx, printSvc, job, state.spools, messageLabel, func() {
				assignBtn.SetLabel(printSpoolButtonLabel(t, job.SpoolID, state.spools))
				if onChanged != nil {
					onChanged()
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
		state.jobs = jobs
		state.spools = spools
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
			return fmt.Sprintf("%s %s", t.SpoolColon, spoolDisplayName(spool))
		}
	}
	return fmt.Sprintf("%s %s", t.SpoolColon, t.UnassignedSpool)
}

func showPrintSpoolDialog(t i18n.T, ctx context.Context, printSvc *service.PrintService, job *domain.PrintJob, spools []domain.Spool, messageLabel *gtk.Label, onSave func()) {
	dialog := gtk.NewWindow()
	dialog.SetTitle(t.AssignHistoryJob)
	dialog.SetDefaultSize(420, 140)
	dialog.SetModal(true)

	box := gtk.NewBox(gtk.OrientationVertical, 10)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)
	dropdown := gtk.NewDropDown(gtk.NewStringList(spoolChoiceNames(t, spools)), nil)
	dropdown.SetSelected(selectedSpoolIndex(job.SpoolID, spools))
	box.Append(dropdown)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel(t.Cancel)
	cancel.ConnectClicked(dialog.Destroy)
	actions.Append(cancel)
	save := gtk.NewButtonWithLabel(t.Save)
	save.ConnectClicked(func() {
		idx := dropdown.Selected()
		spoolID := ""
		if idx > 0 {
			if int(idx-1) >= len(spools) {
				return
			}
			spoolID = spools[idx-1].ID
		}
		if spoolID != job.SpoolID {
			if err := printSvc.ChangeSpool(ctx, job.ID, spoolID); err != nil {
				messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
				messageLabel.SetVisible(true)
				return
			}
			job.SpoolID = spoolID
			messageLabel.SetVisible(false)
		}
		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	})
	actions.Append(save)
	box.Append(actions)
	dialog.SetChild(box)
	dialog.SetVisible(true)
}
