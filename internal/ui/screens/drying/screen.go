package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDryingScreen создаёт экран журнала сушки и рекомендаций.
func NewDryingScreen(ctx context.Context, dryingSvc *service.DryingService, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func()) {
	t := i18n.L()

	box := widgets.NewScreen(t.Drying)

	// Рекомендации по сушке
	box.Append(widgets.NewSectionTitle(t.DryingRecommendations))

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
	box.Append(widgets.NewSectionTitle(t.DryingLog))

	listBox := gtk.NewListBox()
	listBox.SetSelectionMode(gtk.SelectionNone)

	box.Append(widgets.NewScrolled(listBox))

	var refreshDrying = func() {
		widgets.ClearListBox(listBox)

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

	form := newDryingForm(t)
	box.Append(form.root)

	// Выбор катушки
	var spools []domain.Spool
	refreshSpoolChoices := func() {
		notArchived := false
		currentID := ""
		if len(spools) > 0 {
			selected := form.spoolDropdown.Selected()
			if int(selected) < len(spools) {
				currentID = spools[selected].ID
			}
		}

		loaded, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			spools = nil
			form.spoolDropdown.SetModel(gtk.NewStringList([]string{fmt.Sprintf("%s: %v", t.ErrorPrefix, err)}))
			form.spoolDropdown.SetSelected(0)
			return
		}

		spools = loaded
		form.SetSpools(t, spools, currentID)
	}

	refreshScreen := func() {
		refreshSpoolChoices()
		refreshDrying()
	}

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	form.root.Append(saveBtn)

	addBtn.ConnectClicked(func() {
		form.root.SetVisible(!form.root.IsVisible())
	})

	saveBtn.ConnectClicked(func() {
		session, ok := form.Session(spools)
		if !ok {
			return
		}

		if err := dryingSvc.Create(ctx, session); err != nil {
			return
		}

		form.root.SetVisible(false)
		form.Reset()
		refreshDrying()
		if onChanged != nil {
			onChanged()
		}
	})

	refreshScreen()
	return box, refreshScreen
}
