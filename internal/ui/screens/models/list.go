package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
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
	item    *gtk.ListItem
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
		card := gtk.NewBox(gtk.OrientationVertical, 0)
		card.AddCSSClass("card")
		card.SetMarginTop(4)
		card.SetMarginBottom(4)
		card.SetMarginStart(4)
		card.SetMarginEnd(4)
		row := gtk.NewBox(gtk.OrientationHorizontal, 12)
		row.SetMarginTop(12)
		row.SetMarginBottom(12)
		row.SetMarginStart(12)
		row.SetMarginEnd(12)
		picture := gtk.NewPicture()
		picture.SetSizeRequest(widgets.PreviewWidth, widgets.PreviewHeight)
		picture.SetContentFit(gtk.ContentFitContain)
		picture.SetVisible(false)
		row.Append(picture)
		textBox := gtk.NewBox(gtk.OrientationVertical, 4)
		textBox.SetHExpand(true)
		name := gtk.NewLabel("")
		name.AddCSSClass("heading")
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
		printBtn := gtk.NewButtonFromIconName("media-playback-start-symbolic")
		printBtn.SetTooltipText(t.PrintModel)
		printBtn.SetVAlign(gtk.AlignCenter)
		itemNative := item.Native()
		printBtn.ConnectClicked(func() {
			row := rows[itemNative]
			if row == nil {
				return
			}
			position := row.item.Position()
			if int(position) >= len(state.models) {
				return
			}
			selected := state.models[position]
			showPrintLaunchDialog(t, ctx, parent, printerSvc, settingsSvc, state.printers, state.spools, selected.PrinterID, func(req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
				req.SourcePrinterID = selected.PrinterID
				req.GcodeFilename = selected.Path
				return printerSvc.PrintModel(ctx, req)
			}, messageLabel, onChanged)
		})
		row.Append(printBtn)
		card.Append(row)
		item.SetChild(card)
		rows[itemNative] = &modelRowWidgets{item: item, picture: picture, name: name, details: details}
	})

	factory.ConnectBind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		row := rows[item.Native()]
		position := item.Position()
		if row == nil || int(position) >= len(state.models) {
			return
		}
		entry := state.models[position]
		name := entry.Filename
		if name == "" {
			name = entry.Path
		}
		row.name.SetLabel(name)
		details := fmt.Sprintf("%s — %.1f g", state.printerNames[entry.PrinterID], entry.EstimatedWeightG)
		if entry.PrintDuration > 0 {
			details = fmt.Sprintf("%s — %s", details, entry.PrintDuration.String())
		}
		row.details.SetLabel(details)
		widgets.SetPreview(row.picture, entry.PreviewImagePath)
	})
	factory.ConnectUnbind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		if row := rows[item.Native()]; row != nil {
			row.picture.SetPaintable(nil)
			row.picture.SetVisible(false)
		}
	})
	factory.ConnectTeardown(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		delete(rows, item.Native())
	})

	view := gtk.NewListView(selection, &factory.ListItemFactory)
	view.AddCSSClass("card-list")
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
