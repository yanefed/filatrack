package screens

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, dryingSvc *service.DryingService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func()) {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(24)
	box.SetMarginBottom(24)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	t := i18n.L()

	title := gtk.NewLabel(t.Spools)
	title.AddCSSClass("title-1")
	box.Append(title)

	spoolGrid := gtk.NewFlowBox()
	spoolGrid.SetSelectionMode(gtk.SelectionNone)
	spoolGrid.SetHomogeneous(true)
	spoolGrid.SetMinChildrenPerLine(1)
	spoolGrid.SetMaxChildrenPerLine(3)
	spoolGrid.SetColumnSpacing(12)
	spoolGrid.SetRowSpacing(12)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.SetChild(spoolGrid)
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
		spoolGrid.RemoveAll()

		notArchived := false
		spools, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			label := gtk.NewLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			spoolGrid.Append(label)
			return
		}

		for _, spool := range spools {
			_, remaining, err := spoolSvc.GetWithRemaining(ctx, spool.ID)
			if err != nil {
				remaining = 0
			}
			percent := spool.RemainingPercent(spool.TotalWeightG - remaining)
			dryingStatus, dryingErr := dryingSvc.GetStatus(ctx, &spool)

			card := gtk.NewBox(gtk.OrientationVertical, 10)
			card.AddCSSClass("card")
			card.SetMarginTop(2)
			card.SetMarginBottom(2)
			card.SetMarginStart(2)
			card.SetMarginEnd(2)
			card.SetSizeRequest(270, -1)

			content := gtk.NewBox(gtk.OrientationHorizontal, 12)
			content.SetMarginTop(14)
			content.SetMarginStart(14)
			content.SetMarginEnd(14)
			content.Append(widgets.NewSpoolPreview(spool.ColorHex))

			labels := gtk.NewBox(gtk.OrientationVertical, 3)
			labels.SetHExpand(true)

			name := gtk.NewLabel(fmt.Sprintf("%s — %s", spool.DisplayCode, spool.Manufacturer))
			name.AddCSSClass("heading")
			name.SetHAlign(gtk.AlignStart)
			name.SetWrap(true)
			name.SetMaxWidthChars(24)
			labels.Append(name)

			details := gtk.NewLabel(fmt.Sprintf("%s · %s", spool.Color, spool.Material))
			details.AddCSSClass("dim-label")
			details.SetHAlign(gtk.AlignStart)
			details.SetWrap(true)
			labels.Append(details)
			content.Append(labels)
			card.Append(content)

			progress := widgets.NewLevelProgress(percent, fmt.Sprintf(t.SpoolWeightFmt, remaining, spool.TotalWeightG, percent))
			progress.SetMarginStart(14)
			progress.SetMarginEnd(14)
			card.Append(progress)

			weight := gtk.NewLabel(fmt.Sprintf(t.SpoolWeightFmt, remaining, spool.TotalWeightG, percent))
			weight.AddCSSClass("caption")
			weight.AddCSSClass("dim-label")
			weight.SetHAlign(gtk.AlignStart)
			weight.SetMarginStart(14)
			weight.SetMarginEnd(14)
			card.Append(weight)

			card.Append(newSpoolDryingSection(t, dryingStatus, dryingErr))

			actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
			actions.SetHAlign(gtk.AlignEnd)
			actions.SetMarginBottom(12)
			actions.SetMarginStart(14)
			actions.SetMarginEnd(14)

			dryBtn := spoolActionButton("weather-windy-symbolic", t.DrySpool)
			if dryingStatus.NeedsDrying {
				dryBtn.AddCSSClass("warning")
			}
			dryBtn.ConnectClicked(func() {
				showDrySpoolDialog(ctx, parent, dryingSvc, spool, dryingStatus.Profile, func() {
					refreshSpools()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			actions.Append(dryBtn)

			editBtn := spoolActionButton("document-edit-symbolic", t.Edit)
			editBtn.ConnectClicked(func() {
				showEditSpoolDialog(ctx, parent, spoolSvc, settingsSvc, spool, func() {
					refreshSpools()
					if onChanged != nil {
						onChanged()
					}
				})
			})
			actions.Append(editBtn)

			archiveBtn := spoolActionButton("edit-delete-symbolic", t.Archive)
			archiveBtn.AddCSSClass("destructive-action")
			archiveBtn.ConnectClicked(func() {
				if err := spoolSvc.Archive(ctx, spool.ID); err != nil {
					return
				}
				refreshSpools()
				if onChanged != nil {
					onChanged()
				}
			})
			actions.Append(archiveBtn)
			card.Append(actions)

			spoolGrid.Append(card)
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
	colorPicker := widgets.NewColorPicker(t.ColorHexLabel, "")
	formBox.Append(colorPicker.Widget())

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
			ColorHex:     colorPicker.Hex(),
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
		colorPicker.SetHex("")
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

func showEditSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	t := i18n.L()
	dialog := newModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s", t.Edit))
	dialog.SetDefaultSize(360, 320)

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
	colorPicker := widgets.NewColorPicker(t.ColorHexLabel, spool.ColorHex)
	box.Append(colorPicker.Widget())

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
		dialog.Destroy()
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
		updated.ColorHex = colorPicker.Hex()
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

func spoolActionButton(iconName, tooltip string) *gtk.Button {
	button := gtk.NewButtonFromIconName(iconName)
	button.AddCSSClass("circular")
	button.SetTooltipText(tooltip)
	return button
}
