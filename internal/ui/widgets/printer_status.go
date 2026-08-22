package widgets

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// NewPrinterStatus creates a compact status block for a Moonraker printer.
func NewPrinterStatus(t i18n.T, printer domain.Printer) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationVertical, 4)
	row.SetMarginTop(6)
	row.SetMarginBottom(8)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dot := gtk.NewBox(gtk.OrientationHorizontal, 0)
	dot.AddCSSClass("printer-status-dot")
	if PrinterOnline(printer) {
		dot.AddCSSClass("online")
	} else {
		dot.AddCSSClass("offline")
	}
	header.Append(dot)

	progress := ClampProgress(printer.Progress)
	info := gtk.NewLabel(fmt.Sprintf(t.PrinterInfoFmt, printer.Name, string(printer.State), progress*100))
	info.SetHAlign(gtk.AlignStart)
	info.SetHExpand(true)
	info.SetEllipsize(pango.EllipsizeEnd)
	info.SetMaxWidthChars(80)
	header.Append(info)
	row.Append(header)

	if printer.CurrentFile != "" {
		file := gtk.NewLabel(fmt.Sprintf(t.CurrentFileFmt, printer.CurrentFile))
		file.SetHAlign(gtk.AlignStart)
		file.SetHExpand(true)
		file.SetEllipsize(pango.EllipsizeEnd)
		file.SetMaxWidthChars(80)
		file.AddCSSClass("dim-label")
		row.Append(file)
	}

	if PrinterHasActivePrint(printer) {
		bar := gtk.NewProgressBar()
		bar.SetFraction(progress)
		bar.SetShowText(true)
		bar.SetText(fmt.Sprintf("%.0f%%", progress*100))
		row.Append(bar)
	}

	return row
}

func PrinterOnline(printer domain.Printer) bool {
	return printer.State != domain.PrinterStateOffline && printer.State != domain.PrinterStateError
}

func PrinterHasActivePrint(printer domain.Printer) bool {
	return printer.State == domain.PrinterStatePrinting || printer.State == domain.PrinterStatePaused
}

func ClampProgress(progress float64) float64 {
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}
