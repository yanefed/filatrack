package widgets

import (
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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
func NewPrinterStatus(t i18n.T, printer domain.Printer, views ...*layout.View) *PrinterStatusWidget {
	view := layout.Use("printer-status", views...)

	row := layout.Get[*gtk.Box](view, "row")

	dot := layout.Get[*gtk.Box](view, "dot")

	info := layout.Get[*gtk.Label](view, "info")

	file := layout.Get[*gtk.Label](view, "file")

	bar := layout.Get[*gtk.ProgressBar](view, "bar")

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
