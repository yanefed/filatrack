package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

func newPrinterCard(t i18n.T, printer domain.Printer, spools []domain.Spool, onAssign func(uint), onEdit func()) (*gtk.Box, *widgets.PrinterStatusWidget) {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginTop(4)
	card.SetMarginBottom(4)
	card.SetMarginStart(4)
	card.SetMarginEnd(4)

	content := gtk.NewBox(gtk.OrientationVertical, 10)
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	status := widgets.NewPrinterStatus(t, printer)
	content.Append(status)
	endpoint := gtk.NewBox(gtk.OrientationHorizontal, 6)
	endpointIcon := gtk.NewImageFromIconName("network-server-symbolic")
	endpointIcon.AddCSSClass("dim-label")
	endpoint.Append(endpointIcon)
	endpointLabel := gtk.NewLabel(printer.MoonrakerURL)
	endpointLabel.AddCSSClass("caption")
	endpointLabel.AddCSSClass("dim-label")
	endpointLabel.SetHAlign(gtk.AlignStart)
	endpointLabel.SetHExpand(true)
	endpointLabel.SetEllipsize(pango.EllipsizeMiddle)
	endpoint.Append(endpointLabel)
	content.Append(endpoint)

	separator := gtk.NewSeparator(gtk.OrientationHorizontal)
	content.Append(separator)

	assignment := gtk.NewBox(gtk.OrientationHorizontal, 8)
	assignmentLabel := gtk.NewLabel(t.ActiveSpool)
	assignmentLabel.AddCSSClass("dim-label")
	assignmentLabel.SetHAlign(gtk.AlignStart)
	assignment.Append(assignmentLabel)

	dropdown := gtk.NewDropDown(gtk.NewStringList(shared.SpoolChoiceNames(t, spools)), nil)
	dropdown.SetHExpand(true)
	dropdown.SetSelected(shared.SelectedSpoolIndex(printer.ActiveSpoolID, spools))
	dropdown.NotifyProperty("selected", func() {
		if onAssign != nil {
			onAssign(dropdown.Selected())
		}
	})
	assignment.Append(dropdown)

	editButton := gtk.NewButtonFromIconName("document-edit-symbolic")
	editButton.AddCSSClass("circular")
	editButton.SetTooltipText(t.Edit)
	editButton.ConnectClicked(onEdit)
	assignment.Append(editButton)
	content.Append(assignment)

	card.Append(content)
	return card, status
}
