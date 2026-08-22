package dashboard

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newActivePrintsView(t i18n.T, printers []domain.Printer) *gtk.FlowBox {
	grid := gtk.NewFlowBox()
	grid.SetSelectionMode(gtk.SelectionNone)
	grid.SetHomogeneous(true)
	grid.SetMinChildrenPerLine(1)
	grid.SetMaxChildrenPerLine(2)
	grid.SetColumnSpacing(12)
	grid.SetRowSpacing(12)
	if len(printers) == 0 {
		grid.Append(newDashboardStateCard("object-select-symbolic", "success", t.NoActivePrints))
		return grid
	}
	for _, printer := range printers {
		card := gtk.NewBox(gtk.OrientationVertical, 0)
		card.AddCSSClass("card")
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
		grid.Append(card)
	}
	return grid
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
