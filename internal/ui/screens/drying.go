package screens

import (
	"context"
	"fmt"
	"strconv"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDryingScreen создаёт экран журнала сушки и рекомендаций.
func NewDryingScreen(ctx context.Context, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func()) {
	t := i18n.L()

	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	title := gtk.NewLabel(t.DryingTitle)
	title.AddCSSClass("title-1")
	box.Append(title)

	// Рекомендации по сушке
	recLabel := gtk.NewLabel(t.DryingRecommendations)
	recLabel.SetHAlign(gtk.AlignStart)
	recLabel.AddCSSClass("title-3")
	box.Append(recLabel)

	recText := ""
	for _, mat := range []domain.Material{
		domain.MaterialPLA, domain.MaterialPETG, domain.MaterialABS,
		domain.MaterialASA, domain.MaterialTPU, domain.MaterialNylon, domain.MaterialPC,
	} {
		p := dryingSvc.GetRecommendation(ctx, mat)
		if p != nil {
			recText += fmt.Sprintf("%s: %d°C, %.0f %s (%s: %d %s)\n",
				string(p.Material), p.TemperatureC, p.DurationHours, t.Hours,
				t.MaxDaysNoDry, p.MaxDaysNoDry, t.Days)
		}
	}
	recInfo := gtk.NewLabel(recText)
	recInfo.SetHAlign(gtk.AlignStart)
	box.Append(recInfo)

	// Журнал сушки по катушкам
	journalLabel := gtk.NewLabel(t.DryingLog)
	journalLabel.SetHAlign(gtk.AlignStart)
	journalLabel.AddCSSClass("title-3")
	box.Append(journalLabel)

	listBox := gtk.NewListBox()
	listBox.SetSelectionMode(gtk.SelectionNone)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(listBox)
	box.Append(scrolled)

	var refreshDrying func()
	refreshDrying = func() {
		for {
			row := listBox.RowAtIndex(0)
			if row == nil {
				break
			}
			listBox.Remove(row)
		}

		notArchived := false
		spools, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			return
		}

		for _, spool := range spools {
			sessions, err := dryingSvc.ListForSpool(ctx, spool.ID)
			if err != nil {
				continue
			}
			for _, s := range sessions {
				row := gtk.NewLabel(fmt.Sprintf("%s %s (%s) — %d°C, %.1f %s — %s",
					spool.Manufacturer, spool.Color, string(spool.Material),
					s.TemperatureC, s.DurationHours, t.Hours, s.DriedAt.Format("02.01.2006 15:04")))
				row.SetHAlign(gtk.AlignStart)
				listBox.Append(row)
			}
		}
	}

	// Форма добавления сушки
	addBtn := gtk.NewButtonWithLabel(t.AddDrying)
	box.Append(addBtn)

	formBox := gtk.NewBox(gtk.OrientationVertical, 6)
	formBox.SetVisible(false)
	box.Append(formBox)

	// Выбор катушки
	var spools []domain.Spool
	spoolDropdown := gtk.NewDropDown(gtk.NewStringList([]string{t.NoSpoolsAvail}), nil)
	formBox.Append(spoolDropdown)

	refreshSpoolChoices := func() {
		notArchived := false
		currentID := ""
		if len(spools) > 0 {
			selected := spoolDropdown.Selected()
			if int(selected) < len(spools) {
				currentID = spools[selected].ID
			}
		}

		loaded, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			spools = nil
			spoolDropdown.SetModel(gtk.NewStringList([]string{fmt.Sprintf("%s: %v", t.ErrorPrefix, err)}))
			spoolDropdown.SetSelected(0)
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

		spoolDropdown.SetModel(gtk.NewStringList(spoolNames))
		spoolDropdown.SetSelected(selectedIdx)
	}

	refreshScreen := func() {
		refreshSpoolChoices()
		refreshDrying()
	}

	tempEntry := gtk.NewEntry()
	tempEntry.SetPlaceholderText(t.TemperatureC)
	formBox.Append(tempEntry)

	durationEntry := gtk.NewEntry()
	durationEntry.SetPlaceholderText(t.DurationHours)
	formBox.Append(durationEntry)

	notesEntry := gtk.NewEntry()
	notesEntry.SetPlaceholderText(t.Notes)
	formBox.Append(notesEntry)

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	formBox.Append(saveBtn)

	addBtn.ConnectClicked(func() {
		formBox.SetVisible(!formBox.IsVisible())
	})

	saveBtn.ConnectClicked(func() {
		if len(spools) == 0 {
			return
		}
		idx := spoolDropdown.Selected()
		if int(idx) >= len(spools) {
			return
		}

		temp, _ := strconv.Atoi(tempEntry.Text())
		dur, _ := strconv.ParseFloat(durationEntry.Text(), 64)

		session := &domain.DryingSession{
			SpoolID:       spools[idx].ID,
			TemperatureC:  temp,
			DurationHours: dur,
			Notes:         notesEntry.Text(),
		}

		if err := dryingSvc.Create(ctx, session); err != nil {
			return
		}

		formBox.SetVisible(false)
		tempEntry.SetText("")
		durationEntry.SetText("")
		notesEntry.SetText("")
		refreshDrying()
		if onChanged != nil {
			onChanged()
		}
	})

	refreshScreen()
	return box, refreshScreen
}
