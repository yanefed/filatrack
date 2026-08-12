package screens

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"context"
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
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

	var materials []domain.Material
	var materialCombo *gtk.DropDown
	refreshMaterialChoices := func() {
		current := domain.Material("")
		if materialCombo != nil && len(materials) > 0 {
			idx := materialCombo.Selected()
			if int(idx) < len(materials) {
				current = materials[idx]
			}
		}
		loaded, err := settingsSvc.AllMaterials(ctx)
		if err != nil || len(loaded) == 0 {
			loaded = service.BuiltInMaterials
		}
		materials = loaded
		names := make([]string, len(materials))
		selectedIdx := uint(0)
		for i, material := range materials {
			names[i] = string(material)
			if material == current {
				selectedIdx = uint(i)
			}
		}
		if materialCombo != nil {
			materialCombo.SetModel(gtk.NewStringList(names))
			materialCombo.SetSelected(selectedIdx)
		}
	}

	var refreshSpools func()
	refreshSpools = func() {
		refreshMaterialChoices()
		clearListBox(listBox)

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
				spool.DisplayCode, spool.Manufacturer, spool.Color, string(spool.Material),
				remaining, spool.TotalWeightG, percent))
			info.SetHAlign(gtk.AlignStart)
			info.SetHExpand(true)
			row.Append(info)

			editBtn := gtk.NewButtonWithLabel(t.Edit)
			editBtn.ConnectClicked(func() {
				showEditSpoolDialog(ctx, spoolSvc, settingsSvc, spool, func() {
					refreshSpools()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			row.Append(editBtn)

			archiveBtn := gtk.NewButtonWithLabel(t.Archive)
			archiveBtn.ConnectClicked(func() {
				if err := spoolSvc.Archive(ctx, spool.ID); err != nil {
					return
				}
				refreshSpools()
				if onChanged != nil {
					onChanged()
				}
			})
			row.Append(archiveBtn)

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

	materialCombo = gtk.NewDropDown(gtk.NewStringList([]string{}), nil)
	refreshMaterialChoices()
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

	saveBtn.ConnectClicked(func() {
		weight, _ := strconv.ParseFloat(weightEntry.Text(), 64)
		price, _ := strconv.ParseFloat(priceEntry.Text(), 64)
		idx := materialCombo.Selected()
		if int(idx) >= len(materials) {
			return
		}
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

func showEditSpoolDialog(ctx context.Context, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	t := i18n.L()
	dialog := gtk.NewWindow()
	dialog.SetTitle(fmt.Sprintf("%s — %s", t.Edit, spool.DisplayCode))
	dialog.SetDefaultSize(360, 320)
	dialog.SetModal(true)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)

	manufacturerEntry := gtk.NewEntry()
	manufacturerEntry.SetPlaceholderText(t.ManufacturerPH)
	manufacturerEntry.SetText(spool.Manufacturer)
	box.Append(manufacturerEntry)

	materials, err := settingsSvc.AllMaterials(ctx)
	if err != nil || len(materials) == 0 {
		materials = service.BuiltInMaterials
	}
	materialNames := make([]string, len(materials))
	selectedMaterial := uint(0)
	for i, material := range materials {
		materialNames[i] = string(material)
		if material == spool.Material {
			selectedMaterial = uint(i)
		}
	}
	materialCombo := gtk.NewDropDown(gtk.NewStringList(materialNames), nil)
	materialCombo.SetSelected(selectedMaterial)
	box.Append(materialCombo)

	colorEntry := gtk.NewEntry()
	colorEntry.SetPlaceholderText(t.ColorPH)
	colorEntry.SetText(spool.Color)
	box.Append(colorEntry)

	weightEntry := gtk.NewEntry()
	weightEntry.SetPlaceholderText(t.NetWeightPH)
	weightEntry.SetText(fmt.Sprintf("%.0f", spool.TotalWeightG))
	box.Append(weightEntry)

	priceEntry := gtk.NewEntry()
	priceEntry.SetPlaceholderText(t.PricePH)
	priceEntry.SetText(fmt.Sprintf("%.2f", spool.Price))
	box.Append(priceEntry)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(func() {
		dialog.Close()
	})
	actions.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	saveBtn.ConnectClicked(func() {
		idx := materialCombo.Selected()
		if int(idx) >= len(materials) {
			return
		}
		weight, _ := strconv.ParseFloat(weightEntry.Text(), 64)
		price, _ := strconv.ParseFloat(priceEntry.Text(), 64)
		updated := spool
		updated.Manufacturer = manufacturerEntry.Text()
		updated.Material = materials[idx]
		updated.Color = colorEntry.Text()
		updated.TotalWeightG = weight
		updated.Price = price
		if err := spoolSvc.Update(ctx, &updated); err != nil {
			return
		}
		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	})
	actions.Append(saveBtn)
	box.Append(actions)

	dialog.SetChild(box)
	dialog.SetVisible(true)
}
