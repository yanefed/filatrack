package screens

import (
	"awesomeProject/internal/i18n"
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/gcode"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewPrintsScreen создаёт экран истории печатей.
func NewPrintsScreen(ctx context.Context, printSvc *service.PrintService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func()) {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)
	t := i18n.L()

	title := gtk.NewLabel(t.Prints)
	title.AddCSSClass("title-1")
	box.Append(title)

	listBox := gtk.NewListBox()
	listBox.SetSelectionMode(gtk.SelectionNone)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(listBox)
	box.Append(scrolled)

	var refreshPrints func()
	refreshPrints = func() {
		for {
			row := listBox.RowAtIndex(0)
			if row == nil {
				break
			}
			listBox.Remove(row)
		}

		jobs, err := printSvc.List(ctx, repository.PrintJobFilter{})
		if err != nil {
			label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			listBox.Append(label)
			return
		}

		for _, job := range jobs {
			statusIcon := "✅"
			switch job.Status {
			case "failed":
				statusIcon = "❌"
			case "cancelled":
				statusIcon = "🚫"
			}

			row := gtk.NewBox(gtk.OrientationVertical, 4)
			row.SetMarginTop(6)
			row.SetMarginBottom(6)

			header := gtk.NewLabel(fmt.Sprintf("%s %s — %s", statusIcon, job.ModelName, job.GcodeFilename))
			header.SetHAlign(gtk.AlignStart)
			row.Append(header)

			details := gtk.NewLabel(fmt.Sprintf(t.UsageFormat,
				job.ActualWeightG, job.CompletionPercent, job.PrintedAt.Format("02.01.2006 15:04")))
			details.SetHAlign(gtk.AlignStart)
			details.AddCSSClass("dim-label")
			row.Append(details)

			listBox.Append(row)
		}

		if len(jobs) == 0 {
			empty := gtk.NewLabel(t.NoPrintRecords)
			empty.AddCSSClass("dim-label")
			listBox.Append(empty)
		}
	}

	// Форма добавления печати
	addBtn := gtk.NewButtonWithLabel(t.AddPrint)
	box.Append(addBtn)

	formBox := gtk.NewBox(gtk.OrientationVertical, 6)
	formBox.SetVisible(false)
	box.Append(formBox)

	// Путь к gcode файлу
	pathEntry := gtk.NewEntry()
	pathEntry.SetPlaceholderText(t.GCodeFilePath)
	formBox.Append(pathEntry)

	var selectedMeta *domain.GcodeMetadata

	parseBtn := gtk.NewButtonWithLabel(t.LoadGCode)
	formBox.Append(parseBtn)

	// Информация о файле (заполняется после парсинга)
	infoLabel := gtk.NewLabel("")
	infoLabel.SetHAlign(gtk.AlignStart)
	infoLabel.AddCSSClass("dim-label")
	infoLabel.SetVisible(false)
	infoLabel.SetWrap(true)
	formBox.Append(infoLabel)

	// Выбор катушки
	spoolLabel := gtk.NewLabel(t.SpoolColon)
	spoolLabel.SetHAlign(gtk.AlignStart)
	formBox.Append(spoolLabel)

	var spools []domain.Spool
	spoolCombo := gtk.NewDropDown(gtk.NewStringList([]string{t.NoSpoolsAvail}), nil)
	formBox.Append(spoolCombo)

	refreshSpoolChoices := func() {
		notArchived := false
		currentID := ""
		if len(spools) > 0 {
			selected := spoolCombo.Selected()
			if int(selected) < len(spools) {
				currentID = spools[selected].ID
			}
		}

		loaded, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			spools = nil
			spoolCombo.SetModel(gtk.NewStringList([]string{fmt.Sprintf("%s: %v", t.ErrorPrefix, err)}))
			spoolCombo.SetSelected(0)
			return
		}

		spools = loaded
		spoolNames := make([]string, len(spools))
		selectedIdx := uint(0)
		for i, s := range spools {
			spoolNames[i] = fmt.Sprintf("%s %s (%s)", s.Manufacturer, s.Color, string(s.Material))
			if s.ID == currentID {
				selectedIdx = uint(i)
			}
		}
		if len(spoolNames) == 0 {
			spoolNames = []string{t.NoSpoolsAvail}
		}

		spoolCombo.SetModel(gtk.NewStringList(spoolNames))
		spoolCombo.SetSelected(selectedIdx)
	}

	refreshScreen := func() {
		refreshSpoolChoices()
		refreshPrints()
	}

	// Статус печати
	statusLabel := gtk.NewLabel(t.StatusColon)
	statusLabel.SetHAlign(gtk.AlignStart)
	formBox.Append(statusLabel)

	statusNames := []string{t.StatusSuccess, t.StatusCancelled, t.StatusFailed}
	statusValues := []domain.PrintStatus{domain.PrintStatusSuccess, domain.PrintStatusCancelled, domain.PrintStatusFailed}
	statusCombo := gtk.NewDropDown(gtk.NewStringList(statusNames), nil)
	formBox.Append(statusCombo)

	// Процент выполнения
	percentLabel := gtk.NewLabel(t.CompletionPct)
	percentLabel.SetHAlign(gtk.AlignStart)
	formBox.Append(percentLabel)

	percentEntry := gtk.NewEntry()
	percentEntry.SetPlaceholderText("100")
	percentEntry.SetText("100")
	percentEntry.SetSensitive(false)
	formBox.Append(percentEntry)

	// При смене статуса управляем доступностью поля процента
	statusCombo.NotifyProperty("selected", func() {
		idx := statusCombo.Selected()
		if statusValues[idx] == domain.PrintStatusSuccess {
			percentEntry.SetText("100")
			percentEntry.SetSensitive(false)
		} else {
			percentEntry.SetSensitive(true)
		}
	})

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	formBox.Append(saveBtn)

	addBtn.ConnectClicked(func() {
		formBox.SetVisible(!formBox.IsVisible())
	})

	parseBtn.ConnectClicked(func() {
		path := pathEntry.Text()
		if path == "" {
			infoLabel.SetLabel(t.EnterFilePath)
			infoLabel.SetVisible(true)
			selectedMeta = nil
			return
		}

		meta, err := gcode.ParseFile(path)
		if err != nil {
			infoLabel.SetLabel(fmt.Sprintf("%s: %v", t.ParseError, err))
			infoLabel.SetVisible(true)
			selectedMeta = nil
			return
		}
		if meta.Filename == "" {
			meta.Filename = filepath.Base(path)
		}
		selectedMeta = meta

		info := fmt.Sprintf(t.ModelInfo,
			meta.ModelName, meta.Slicer, meta.FilamentWeightG, meta.FilamentLengthMM, meta.PrintDuration)
		infoLabel.SetLabel(info)
		infoLabel.SetVisible(true)
	})

	saveBtn.ConnectClicked(func() {
		if selectedMeta == nil {
			return
		}
		if len(spools) == 0 {
			return
		}

		idx := spoolCombo.Selected()
		if int(idx) >= len(spools) {
			return
		}
		spoolID := spools[idx].ID

		statusIdx := statusCombo.Selected()
		status := statusValues[statusIdx]

		percent := 100
		if status != domain.PrintStatusSuccess {
			p, err := strconv.Atoi(percentEntry.Text())
			if err == nil && p >= 0 && p <= 100 {
				percent = p
			}
		}

		_, err := printSvc.CreateFromGcode(ctx, spoolID, selectedMeta, status, percent)
		if err != nil {
			infoLabel.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
			infoLabel.SetVisible(true)
			return
		}

		formBox.SetVisible(false)
		selectedMeta = nil
		pathEntry.SetText("")
		infoLabel.SetVisible(false)
		percentEntry.SetText("100")
		statusCombo.SetSelected(0)
		refreshPrints()
		if onChanged != nil {
			onChanged()
		}
	})

	refreshScreen()
	return box, refreshScreen
}
