package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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

func newVirtualModelsList(t i18n.T, ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, settingsSvc *service.SettingsService, messageLabel *gtk.Label, onChanged func(), views ...*layout.View) (*gtk.ListView, func([]domain.PrinterModelFile, []domain.Printer, []domain.Spool)) {
	view := layout.Use("model-list", views...)

	state := &modelListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*modelRowWidgets)

	factory.ConnectSetup(func(object *coreglib.Object) {
		view := layout.Load("model-row")

		item := object.Cast().(*gtk.ListItem)
		item.SetActivatable(false)
		item.SetFocusable(false)
		card := layout.Get[*gtk.Box](view, "card")

		picture := layout.Get[*gtk.Picture](view, "picture")

		name := layout.Get[*gtk.Label](view, "name")

		details := layout.Get[*gtk.Label](view, "details")

		printBtn := layout.Get[*gtk.Button](view, "printBtn")
		printBtn.SetTooltipText(t.PrintModel)

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

	listView := layout.Configure[*gtk.ListView](view, "view", func(w *gtk.ListView) { w.SetModel(selection); w.SetFactory(&factory.ListItemFactory) })

	update := func(models []domain.PrinterModelFile, printers []domain.Printer, spools []domain.Spool) {
		state.models = models
		state.printers = printers
		state.spools = spools
		state.printerNames = printerNamesByID(printers)
		model.Splice(0, model.NItems(), modelKeys(models))
	}
	return listView, update
}

func modelKeys(models []domain.PrinterModelFile) []string {
	keys := make([]string, len(models))
	for i := range models {
		keys[i] = models[i].PrinterID + "\x00" + models[i].Path
	}
	return keys
}
