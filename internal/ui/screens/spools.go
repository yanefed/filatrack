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

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, spoolSvc *service.SpoolService, onChanged func()) (*gtk.Box, func()) {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	t := i18n.L()

	title := gtk.NewLabel(t.SpoolsScreenTitle)
	title.AddCSSClass("title-1")
	box.Append(title)

	listBox := gtk.NewListBox()
	listBox.SetSelectionMode(gtk.SelectionNone)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(listBox)
	box.Append(scrolled)

	refreshSpools := func() {
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
			label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			listBox.Append(label)
			return
		}

		for _, spool := range spools {
			_, remaining, err := spoolSvc.GetWithRemaining(ctx, spool.ID)
			if err != nil {
				remaining = 0
			}
			percent := spool.RemainingPercent(spool.TotalWeightG - remaining)

			row := gtk.NewBox(gtk.OrientationHorizontal, 12)
			row.SetMarginTop(6)
			row.SetMarginBottom(6)

			info := gtk.NewLabel(fmt.Sprintf(t.SpoolInfoFmt,
				spool.Manufacturer, spool.Color, string(spool.Material),
				remaining, spool.TotalWeightG, percent))
			info.SetHAlign(gtk.AlignStart)
			info.SetHExpand(true)
			row.Append(info)

			listBox.Append(row)
		}
	}

	// Форма добавления
	addBtn := gtk.NewButtonWithLabel(t.AddSpoolBtn)
	box.Append(addBtn)

	formBox := gtk.NewBox(gtk.OrientationVertical, 6)
	formBox.SetVisible(false)
	box.Append(formBox)

	manufacturerEntry := gtk.NewEntry()
	manufacturerEntry.SetPlaceholderText(t.ManufacturerPH)
	formBox.Append(manufacturerEntry)

	materialCombo := gtk.NewDropDown(gtk.NewStringList([]string{
		"PLA", "PETG", "ABS", "ASA", "TPU", "Nylon", "PC",
	}), nil)
	formBox.Append(materialCombo)

	colorEntry := gtk.NewEntry()
	colorEntry.SetPlaceholderText(t.ColorPH)
	formBox.Append(colorEntry)

	weightEntry := gtk.NewEntry()
	weightEntry.SetPlaceholderText(t.NetWeightPH)
	formBox.Append(weightEntry)

	priceEntry := gtk.NewEntry()
	priceEntry.SetPlaceholderText(t.PricePH)
	formBox.Append(priceEntry)

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	formBox.Append(saveBtn)

	addBtn.ConnectClicked(func() {
		formBox.SetVisible(!formBox.IsVisible())
	})

	materials := []domain.Material{
		domain.MaterialPLA, domain.MaterialPETG, domain.MaterialABS,
		domain.MaterialASA, domain.MaterialTPU, domain.MaterialNylon, domain.MaterialPC,
	}

	saveBtn.ConnectClicked(func() {
		weight, _ := strconv.ParseFloat(weightEntry.Text(), 64)
		price, _ := strconv.ParseFloat(priceEntry.Text(), 64)
		idx := materialCombo.Selected()
		mat := materials[idx]

		spool := &domain.Spool{
			Manufacturer: manufacturerEntry.Text(),
			Material:     mat,
			Color:        colorEntry.Text(),
			ColorHex:     "#000000",
			DiameterMM:   1.75,
			TotalWeightG: weight,
			Price:        price,
		}

		if err := spoolSvc.Create(ctx, spool); err != nil {
			return
		}

		formBox.SetVisible(false)
		manufacturerEntry.SetText("")
		colorEntry.SetText("")
		weightEntry.SetText("")
		priceEntry.SetText("")
		refreshSpools()
		if onChanged != nil {
			onChanged()
		}
	})

	refreshSpools()
	return box, refreshSpools
}
