package dashboard

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newActivePrintsView(t i18n.T, printers []domain.Printer) (*gtk.Box, []*widgets.PrinterStatusWidget) {
	list := gtk.NewBox(gtk.OrientationVertical, 8)
	list.SetHExpand(true)
	if len(printers) == 0 {
		list.Append(newDashboardStateCard("object-select-symbolic", "success", t.NoActivePrints))
		return list, nil
	}
	statuses := make([]*widgets.PrinterStatusWidget, 0, len(printers))
	for _, printer := range printers {
		card := gtk.NewBox(gtk.OrientationVertical, 0)
		card.AddCSSClass("card")
		card.SetHAlign(gtk.AlignFill)
		card.SetHExpand(true)
		card.SetMarginTop(2)
		card.SetMarginBottom(2)
		card.SetMarginStart(2)
		card.SetMarginEnd(2)
		status := widgets.NewPrinterStatus(t, printer)
		status.SetMarginTop(8)
		status.SetMarginBottom(8)
		status.SetMarginStart(12)
		status.SetMarginEnd(12)
		card.Append(status)
		list.Append(card)
		statuses = append(statuses, status)
	}
	return list, statuses
}

func newDashboardStateCard(iconName, colorClass, text string) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationHorizontal, 10)
	card.AddCSSClass("card")
	card.SetMarginTop(2)
	card.SetMarginBottom(2)
	card.SetMarginStart(2)
	card.SetMarginEnd(2)
	icon := gtk.NewImageFromIconName(iconName)
	icon.AddCSSClass(colorClass)
	icon.SetMarginTop(14)
	icon.SetMarginBottom(14)
	icon.SetMarginStart(14)
	card.Append(icon)
	label := gtk.NewLabel(text)
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	label.SetMarginTop(14)
	label.SetMarginBottom(14)
	label.SetMarginEnd(14)
	card.Append(label)
	return card
}
