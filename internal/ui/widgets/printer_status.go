package widgets

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// PrinterStatusWidget keeps its GTK objects alive and updates their contents in place.
type PrinterStatusWidget struct {
	*gtk.Box
	dot      *gtk.Box
	info     *gtk.Label
	file     *gtk.Label
	progress *gtk.ProgressBar

	lastInfo     string
	lastFile     string
	lastProgress float64
	lastOnline   bool
	lastHasFile  bool
	lastHasPrint bool
	initialized  bool
}

// NewPrinterStatus creates a compact status block for a Moonraker printer.
func NewPrinterStatus(t i18n.T, printer domain.Printer) *PrinterStatusWidget {
	row := gtk.NewBox(gtk.OrientationVertical, 4)
	row.SetMarginTop(6)
	row.SetMarginBottom(8)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dot := gtk.NewBox(gtk.OrientationHorizontal, 0)
	dot.AddCSSClass("printer-status-dot")
	header.Append(dot)

	info := gtk.NewLabel("")
	info.AddCSSClass("heading")
	info.SetHAlign(gtk.AlignStart)
	info.SetHExpand(true)
	info.SetEllipsize(pango.EllipsizeEnd)
	info.SetMaxWidthChars(80)
	header.Append(info)
	row.Append(header)

	file := gtk.NewLabel("")
	file.SetHAlign(gtk.AlignStart)
	file.SetHExpand(true)
	file.SetEllipsize(pango.EllipsizeEnd)
	file.SetMaxWidthChars(80)
	file.AddCSSClass("dim-label")
	row.Append(file)

	bar := gtk.NewProgressBar()
	bar.SetShowText(true)
	row.Append(bar)

	status := &PrinterStatusWidget{Box: row, dot: dot, info: info, file: file, progress: bar}
	status.Update(t, printer)
	return status
}

// Update changes only the properties whose displayed values have changed.
func (status *PrinterStatusWidget) Update(t i18n.T, printer domain.Printer) {
	infoText := PrinterInfoText(t, printer)
	fileText := ""
	hasFile := printer.CurrentFile != ""
	if hasFile {
		fileText = fmt.Sprintf(t.CurrentFileFmt, printer.CurrentFile)
	}
	progress := ClampProgress(printer.Progress)
	hasPrint := PrinterHasActivePrint(printer)
	online := PrinterOnline(printer)

	if !status.initialized || infoText != status.lastInfo {
		status.info.SetLabel(infoText)
		status.lastInfo = infoText
	}
	if !status.initialized || fileText != status.lastFile {
		status.file.SetLabel(fileText)
		status.lastFile = fileText
	}
	if !status.initialized || hasFile != status.lastHasFile {
		status.file.SetVisible(hasFile)
		status.lastHasFile = hasFile
	}
	if !status.initialized || progress != status.lastProgress {
		status.progress.SetFraction(progress)
		status.progress.SetText(fmt.Sprintf("%.0f%%", progress*100))
		status.lastProgress = progress
	}
	if !status.initialized || hasPrint != status.lastHasPrint {
		status.progress.SetVisible(hasPrint)
		status.lastHasPrint = hasPrint
	}
	if !status.initialized || online != status.lastOnline {
		if online {
			status.dot.RemoveCSSClass("offline")
			status.dot.AddCSSClass("online")
		} else {
			status.dot.RemoveCSSClass("online")
			status.dot.AddCSSClass("offline")
		}
		status.lastOnline = online
	}
	status.initialized = true
}

func PrinterOnline(printer domain.Printer) bool {
	return printer.State != domain.PrinterStateOffline && printer.State != domain.PrinterStateError
}

func PrinterHasActivePrint(printer domain.Printer) bool {
	return printer.State == domain.PrinterStatePrinting || printer.State == domain.PrinterStatePaused
}

func PrinterInfoText(t i18n.T, printer domain.Printer) string {
	if PrinterHasActivePrint(printer) {
		return fmt.Sprintf(t.PrinterInfoFmt, printer.Name, string(printer.State), ClampProgress(printer.Progress)*100)
	}
	return fmt.Sprintf(t.PrinterIdleInfoFmt, printer.Name, string(printer.State))
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
