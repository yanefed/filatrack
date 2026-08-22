package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolsScreen создаёт экран управления катушками филамента.
func NewSpoolsScreen(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, dryingSvc *service.DryingService, settingsSvc *service.SettingsService, onChanged func()) (*gtk.Box, func(), func()) {
	t := i18n.L()
	box := widgets.NewScreen(t.Spools)

	spoolGrid := gtk.NewFlowBox()
	spoolGrid.SetSelectionMode(gtk.SelectionNone)
	spoolGrid.SetHomogeneous(true)
	spoolGrid.SetMinChildrenPerLine(1)
	spoolGrid.SetMaxChildrenPerLine(3)
	spoolGrid.SetColumnSpacing(12)
	spoolGrid.SetRowSpacing(12)

	scrolled := widgets.NewScrolled(spoolGrid)
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
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

			spoolGrid.Append(newSpoolCard(t, spool, remaining, percent, dryingStatus, dryingErr, spoolCardActions{
				dry: func() {
					showDrySpoolDialog(ctx, parent, dryingSvc, spool, dryingStatus.Profile, func() {
						refreshSpools()
						if onChanged != nil {
							onChanged()
						}
					})
				},
				edit: func() {
					showEditSpoolDialog(ctx, parent, spoolSvc, settingsSvc, spool, func() {
						refreshSpools()
						if onChanged != nil {
							onChanged()
						}
					})
				},
				archive: func() {
					if err := spoolSvc.Archive(ctx, spool.ID); err != nil {
						return
					}
					refreshSpools()
					if onChanged != nil {
						onChanged()
					}
				},
			}))
		}
	}

	form := newSpoolForm(t, materials, nil)
	formBox := form.Widget()
	formBox.SetVisible(false)
	box.Append(formBox)
	materialCombo = form.materialCombo
	refreshMaterialChoices()

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	formBox.Append(saveBtn)

	toggleAddForm := func() {
		formBox.SetVisible(!formBox.IsVisible())
		if formBox.IsVisible() {
			form.manufacturerEntry.GrabFocus()
		}
	}

	saveBtn.ConnectClicked(func() {
		spool, ok := form.Spool(materials, domain.Spool{})
		if !ok {
			return
		}
		if err := spoolSvc.Create(ctx, &spool); err != nil {
			return
		}

		formBox.SetVisible(false)
		form.Reset()
		refreshSpools()
		if onChanged != nil {
			onChanged()
		}
	})

	refreshSpools()
	return box, refreshSpools, toggleAddForm
}

func showEditSpoolDialog(ctx context.Context, parent *gtk.Window, spoolSvc *service.SpoolService, settingsSvc *service.SettingsService, spool domain.Spool, onSave func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s", t.Edit))
	dialog.SetDefaultSize(360, 320)

	materials, err := settingsSvc.AllMaterials(ctx)
	if err != nil || len(materials) == 0 {
		materials = service.BuiltInMaterials
	}
	box := widgets.NewDialogContent(8)
	form := newSpoolForm(t, materials, &spool)
	box.Append(form.Widget())

	actions := widgets.NewActions(8, gtk.AlignFill)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(func() {
		dialog.Destroy()
	})
	actions.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	saveBtn.ConnectClicked(func() {
		updated, ok := form.Spool(materials, spool)
		if !ok {
			return
		}
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
