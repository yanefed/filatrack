package screens

import (
	"context"
	"fmt"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// NewPrintsScreen создаёт экран истории печатей.
func NewPrintsScreen(ctx context.Context, printSvc *service.PrintService, printerSvc *service.PrinterService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func(), func()) {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)
	t := i18n.L()

	title := gtk.NewLabel(t.Prints)
	title.AddCSSClass("title-1")
	box.Append(title)

	activeList := newPrintsListBox()
	listBox := newPrintsListBox()

	messageLabel := gtk.NewLabel("")
	messageLabel.SetHAlign(gtk.AlignStart)
	messageLabel.AddCSSClass("dim-label")
	messageLabel.SetWrap(true)
	messageLabel.SetVisible(false)
	box.Append(messageLabel)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	listContainer := gtk.NewBox(gtk.OrientationVertical, 0)
	listContainer.Append(activeList)
	listContainer.Append(listBox)
	scrolled.SetChild(listContainer)
	box.Append(scrolled)

	var printers []domain.Printer
	activePrintCount := 0
	printerCombo := gtk.NewDropDown(gtk.NewStringList([]string{t.NoPrinters}), nil)
	box.Append(printerCombo)

	renderActivePrints := func() int {
		clearListBox(activeList)
		rendered := 0
		for _, printer := range printers {
			if !printerHasActivePrint(printer) {
				continue
			}
			if len(printers) > 0 {
				idx := printerCombo.Selected()
				if int(idx) < len(printers) && printers[idx].ID != printer.ID {
					continue
				}
			}
			activeList.Append(printerStatusWidget(t, printer))
			rendered++
		}
		return rendered
	}

	var refreshPrints func()
	refreshLivePrints := func() {
		loaded, err := printerSvc.List(ctx)
		if err != nil {
			return
		}
		printers = loaded
		previousActiveCount := activePrintCount
		activePrintCount = renderActivePrints()
		if previousActiveCount > 0 && activePrintCount == 0 {
			refreshPrints()
		}
	}
	refreshPrints = func() {
		oldListBox := listBox
		listBox = newPrintsListBox()
		listContainer.Remove(oldListBox)
		listContainer.Append(listBox)
		if oldListBox != nil {
			clearListBox(oldListBox)
		}

		filter := repository.PrintJobFilter{}
		if len(printers) > 0 {
			idx := printerCombo.Selected()
			if int(idx) < len(printers) {
				printerID := printers[idx].ID
				filter.PrinterID = &printerID
			}
		}
		filter.Limit = printsRenderLimit
		jobs, err := printSvc.List(ctx, filter)
		if err != nil {
			label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			listBox.Append(label)
			return
		}
		currentSpools := activeSpools(ctx, spoolSvc)
		activePrintCount = renderActivePrints()

		for _, job := range jobs {
			job := job
			statusIcon := "✅"
			switch job.Status {
			case "failed":
				statusIcon = "❌"
			case "cancelled":
				statusIcon = "🚫"
			}

			row := gtk.NewBox(gtk.OrientationHorizontal, 8)
			row.SetMarginTop(6)
			row.SetMarginBottom(6)

			if preview := newScaledPreview(job.PreviewImagePath); preview != nil {
				row.Append(preview)
			}

			content := gtk.NewBox(gtk.OrientationVertical, 4)
			content.SetHExpand(true)
			row.Append(content)

			title := job.ModelName
			if title == "" {
				title = job.GcodeFilename
			}
			header := gtk.NewLabel(fmt.Sprintf("%s %s", statusIcon, title))
			header.SetHAlign(gtk.AlignStart)
			header.SetHExpand(true)
			header.SetEllipsize(pango.EllipsizeEnd)
			header.SetMaxWidthChars(80)
			content.Append(header)

			details := gtk.NewLabel(printJobUsageText(t, job))
			details.SetHAlign(gtk.AlignStart)
			details.AddCSSClass("dim-label")
			content.Append(details)

			if len(currentSpools) > 0 {
				assignRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
				assignLabel := gtk.NewLabel(t.SpoolColon)
				assignLabel.SetHAlign(gtk.AlignStart)
				assignRow.Append(assignLabel)

				dropdown := gtk.NewDropDown(gtk.NewStringList(spoolChoiceNames(t, currentSpools)), nil)
				updatingDropdown := true
				dropdown.SetSelected(selectedSpoolIndex(job.SpoolID, currentSpools))
				updatingDropdown = false
				assignRow.Append(dropdown)

				dropdown.NotifyProperty("selected", func() {
					if updatingDropdown {
						return
					}
					idx := dropdown.Selected()
					spoolID := ""
					if idx > 0 {
						if int(idx-1) >= len(currentSpools) {
							return
						}
						spoolID = currentSpools[idx-1].ID
					}
					if spoolID == job.SpoolID {
						return
					}
					if err := printSvc.ChangeSpool(ctx, job.ID, spoolID); err != nil {
						messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
						messageLabel.SetVisible(true)
						return
					}
					job.SpoolID = spoolID
					messageLabel.SetVisible(false)
					if onChanged != nil {
						onChanged()
					}
				})
				content.Append(assignRow)
			}

			listBox.Append(row)
		}

		if activePrintCount == 0 && len(jobs) == 0 {
			empty := gtk.NewLabel(t.NoPrintRecords)
			empty.AddCSSClass("dim-label")
			listBox.Append(empty)
		}
	}

	refreshPrinterChoices := func() {
		currentID := ""
		if len(printers) > 0 {
			selected := printerCombo.Selected()
			if int(selected) < len(printers) {
				currentID = printers[selected].ID
			}
		}

		loaded, err := printerSvc.List(ctx)
		if err != nil {
			printers = nil
			printerCombo.SetModel(gtk.NewStringList([]string{fmt.Sprintf("%s: %v", t.ErrorPrefix, err)}))
			printerCombo.SetSelected(0)
			return
		}

		printers = loaded
		printerNames := make([]string, len(printers))
		selectedIdx := uint(0)
		for i, printer := range printers {
			printerNames[i] = printer.Name
			if printer.ID == currentID {
				selectedIdx = uint(i)
			}
		}
		if len(printerNames) == 0 {
			printerNames = []string{t.NoPrinters}
		}
		printerCombo.SetModel(gtk.NewStringList(printerNames))
		printerCombo.SetSelected(selectedIdx)
		activePrintCount = renderActivePrints()
	}

	refreshScreen := func() {
		refreshPrinterChoices()
		refreshPrints()
	}

	syncHistoryBtn := gtk.NewButtonWithLabel(t.RefreshHistory)
	box.Append(syncHistoryBtn)
	syncHistoryBtn.ConnectClicked(func() {
		if len(printers) == 0 {
			return
		}
		printerIdx := printerCombo.Selected()
		if int(printerIdx) >= len(printers) {
			return
		}
		printerID := printers[printerIdx].ID
		syncHistoryBtn.SetSensitive(false)
		messageLabel.SetLabel("Loading…")
		messageLabel.SetVisible(true)
		var created int
		var err error
		runAsyncUI(func() {
			created, err = printerSvc.SyncPrinterHistory(ctx, printerID, 0)
		}, func() {
			syncHistoryBtn.SetSensitive(true)
			if err != nil {
				messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
				messageLabel.SetVisible(true)
				return
			}
			messageLabel.SetLabel(fmt.Sprintf("%s: %d", t.RefreshHistory, created))
			messageLabel.SetVisible(true)
			refreshPrints()
			if onChanged != nil {
				onChanged()
			}
		})
	})

	refreshScreen()
	return box, refreshScreen, refreshLivePrints
}

func newPrintsListBox() *gtk.ListBox {
	list := gtk.NewListBox()
	list.SetSelectionMode(gtk.SelectionNone)
	return list
}

func printJobUsageText(t i18n.T, job domain.PrintJob) string {
	printedAt := job.PrintedAt.Format("02.01.2006 15:04")
	if job.ActualWeightG > 0 || job.EstimatedLengthMM <= 0 {
		return fmt.Sprintf(t.UsageFormat, job.ActualWeightG, job.CompletionPercent, printedAt)
	}
	return fmt.Sprintf("Usage: %.0f mm | Completion: %d%% | %s", job.EstimatedLengthMM, job.CompletionPercent, printedAt)
}

func selectedDirectSpool(dropdown *gtk.DropDown, spools []domain.Spool) string {
	idx := dropdown.Selected()
	if int(idx) >= len(spools) {
		return ""
	}
	return spools[idx].ID
}

func showLaunchResult(t i18n.T, label *gtk.Label, result *domain.PrintLaunchResult, err error) {
	if err != nil {
		label.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
		label.SetVisible(true)
		return
	}
	if result == nil {
		return
	}
	switch {
	case result.Filament.Blocked:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchBlocked, result.Filament.Warning))
	case result.RequiresUserConfirmation:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchWarning, result.Filament.Warning))
	case result.Uploaded:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchUploaded, result.Filename))
	case result.Started || result.Queued:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchStarted, result.Filename))
	default:
		label.SetLabel(fmt.Sprintf("%s: %.1fg / %.1fg", t.LaunchWarning, result.Filament.RequiredWeightG, result.Filament.RemainingBeforeG))
	}
	label.SetVisible(true)
}

func showGCodeFileChooser(pathEntry *gtk.Entry) {
	t := i18n.L()
	dialog := gtk.NewFileChooserNative(t.SelectGCode, nil, gtk.FileChooserActionOpen, t.SelectGCode, t.Cancel)

	filter := gtk.NewFileFilter()
	filter.AddSuffix("gcode")
	filter.AddSuffix("gco")
	filter.AddSuffix("gc")
	filter.AddPattern("*.gcode")
	filter.AddPattern("*.GCODE")
	filter.AddPattern("*.gco")
	filter.AddPattern("*.GCO")
	filter.AddPattern("*.gc")
	filter.AddPattern("*.GC")
	dialog.AddFilter(filter)
	dialog.SetFilter(filter)

	dialog.ConnectResponse(func(responseID int) {
		defer dialog.Destroy()
		if gtk.ResponseType(responseID) != gtk.ResponseAccept {
			return
		}

		file := dialog.File()
		if file == nil {
			return
		}

		path := file.Path()
		if path == "" {
			path = file.ParseName()
		}
		pathEntry.SetText(path)
	})

	dialog.Show()
}
