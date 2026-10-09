package dashboard

import (
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newActivePrintsView(t i18n.T, printers []domain.Printer) (*gtk.Box, []*widgets.PrinterStatusWidget) {
	view := layout.Load("active-printers")

	list := layout.Get[*gtk.Box](view, "list")

	if len(printers) == 0 {
		list.Append(newDashboardStateCard("object-select-symbolic", "success", t.NoActivePrints))
		return list, nil
	}
	statuses := make([]*widgets.PrinterStatusWidget, 0, len(printers))
	for _, printer := range printers {
		view := layout.Load("dashboard-active-card")

		card := layout.Get[*gtk.Box](view, "card")

		status := widgets.NewPrinterStatus(t, printer, layout.Scope(view, "card_slot_1"))

		list.Append(card)
		statuses = append(statuses, status)
	}
	return list, statuses
}

func newDashboardStateCard(iconName, colorClass, text string) *gtk.Box {
	view := layout.Load("dashboard-state-card")

	card := layout.Get[*gtk.Box](view, "card")

	icon := layout.Configure[*gtk.Image](view, "icon", func(w *gtk.Image) { w.SetFromIconName(iconName) })
	icon.AddCSSClass(colorClass)
	_ = layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(text) })

	return card
}
