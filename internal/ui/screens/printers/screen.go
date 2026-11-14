package printers

import (
	"context"
	"fmt"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

// NewPrintersScreen creates the Moonraker printer management screen.
func NewPrintersScreen(ctx context.Context, parent *gtk.Window, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func(), views ...*layout.View) (*gtk.Box, func(), func(), *gtk.Button) {
	view := layout.Use("printers-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Printers) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	emptyLabel := layout.Configure[*gtk.Label](view, "emptyLabel", func(w *gtk.Label) { w.SetLabel(t.NoPrinters) })

	list := layout.Get[*gtk.ListBox](view, "list")

	type card struct {
		root     *gtk.Box
		status   *widgets.PrinterStatusWidget
		view     *layout.View
		printer  domain.Printer
		spools   []domain.Spool
		updating bool
		names    *gtk.StringList
	}
	cards := make(map[string]*card)
	var order []string
	type result struct {
		printers []domain.Printer
		spools   []domain.Spool
		err      error
		live     bool
	}
	loader := shared.NewLoader[result](ctx, box)
	fullPending := false
	var refreshPrinters func()
	apply := func(data result) {
		if data.err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, data.err))
			messageLabel.SetVisible(true)
			return
		}
		ids := make([]string, len(data.printers))
		for i, p := range data.printers {
			ids[i] = p.ID
		}
		if data.live && !slices.Equal(order, ids) {
			refreshPrinters()
			return
		}
		if !data.live {
			fullPending = false
		}
		for _, printer := range data.printers {
			row := cards[printer.ID]
			if row == nil {
				row = &card{view: layout.Load("printer-card"), printer: printer, spools: data.spools}
				cards[printer.ID] = row
				assign := shared.Action(ctx, box)
				row.root, row.status = newPrinterCard(t, printer, data.spools, func(index uint) {
					if row.updating {
						return
					}
					id := row.printer.ID
					spoolID := ""
					if index > 0 && int(index-1) < len(row.spools) {
						spoolID = row.spools[index-1].ID
					}
					row.root.SetSensitive(false)
					assign(func() error {
						if spoolID == "" {
							return printerSvc.ClearAssignedSpool(ctx, id)
						}
						return printerSvc.AssignSpool(ctx, id, spoolID)
					}, func(err error) {
						row.root.SetSensitive(true)
						refreshPrinters()
						if err != nil {
							messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.AssignPrinterError, err))
							messageLabel.SetVisible(true)
							return
						}
						if onChanged != nil {
							onChanged()
						}
					})
				}, func() {
					showEditPrinterDialog(ctx, parent, printerSvc, row.printer, func() {
						refreshPrinters()
						if onChanged != nil {
							onChanged()
						}
					})
				}, row.view)
			}
			row.printer = printer
			row.status.Update(t, printer)
			if !data.live {
				row.updating = true
				row.spools = data.spools
				layout.Get[*gtk.Label](row.view, "endpointLabel").SetLabel(printer.MoonrakerURL)
				dropdown := layout.Get[*gtk.DropDown](row.view, "dropdown")
				// Retain the dropdown model, selection and signal handlers.
				if row.names == nil {
					row.names = gtk.NewStringList(nil)
					dropdown.SetModel(row.names)
				}
				shared.SyncStrings(row.names, shared.SpoolChoiceNames(t, data.spools))
				dropdown.SetSelected(shared.SelectedSpoolIndex(printer.ActiveSpoolID, data.spools))
				row.updating = false
			}
		}
		if !slices.Equal(order, ids) {
			widgets.ClearListBox(list)
			live := make(map[string]bool, len(ids))
			for _, id := range ids {
				live[id] = true
				list.Append(cards[id].root)
			}
			for id := range cards {
				if !live[id] {
					delete(cards, id)
				}
			}
			order = ids
		}
		emptyLabel.SetVisible(len(ids) == 0)
	}
	refreshPrinters = func() {
		fullPending = true
		loader.Request(func(ctx context.Context) result {
			p, e := printerSvc.List(ctx)
			if e != nil {
				return result{err: e}
			}
			archived := false
			s, e := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &archived})
			return result{printers: p, spools: s, err: e}
		}, func(data result) { fullPending = false; apply(data) })
	}
	refreshPrinterStatuses := func() {
		if fullPending {
			return
		}
		loader.Poll(func(ctx context.Context) result {
			p, e := printerSvc.List(ctx)
			return result{printers: p, err: e, live: true}
		}, apply)
	}

	addPrinterButton := layout.Get[*gtk.Button](view, "addPrinterButton")
	addPrinterButton.SetTooltipText(t.AddPrinter)
	addPrinterButton.ConnectClicked(func() {
		showAddPrinterDialog(ctx, parent, printerSvc, func() {
			refreshPrinters()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	refreshPrinters()
	return box, refreshPrinters, refreshPrinterStatuses, addPrinterButton
}
