package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newPrinterCard(t i18n.T, printer domain.Printer, spools []domain.Spool, onAssign func(uint), onEdit func()) (*gtk.Box, *widgets.PrinterStatusWidget) {
	view := layout.Load("printer-card")

	card := layout.Get[*gtk.Box](view, "card")

	status := widgets.NewPrinterStatus(t, printer, layout.Scope(view, "content_slot_1"))

	_ = layout.Configure[*gtk.Label](view, "endpointLabel", func(w *gtk.Label) { w.SetLabel(printer.MoonrakerURL) })
	_ = layout.Configure[*gtk.Label](view, "assignmentLabel", func(w *gtk.Label) { w.SetLabel(t.ActiveSpool) })

	dropdown := layout.Configure[*gtk.DropDown](view, "dropdown", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList(shared.SpoolChoiceNames(t, spools))) })

	dropdown.SetSelected(shared.SelectedSpoolIndex(printer.ActiveSpoolID, spools))
	dropdown.NotifyProperty("selected", func() {
		if onAssign != nil {
			onAssign(dropdown.Selected())
		}
	})

	editButton := layout.Get[*gtk.Button](view, "editButton")

	editButton.SetTooltipText(t.Edit)
	editButton.ConnectClicked(onEdit)

	return card, status
}
