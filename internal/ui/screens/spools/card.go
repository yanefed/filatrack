package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type spoolCardActions struct {
	dry     func()
	edit    func()
	archive func()
}

func newSpoolCard(t i18n.T, spool domain.Spool, remaining, percent float64, dryingStatus service.DryingStatus, dryingErr error, handlers spoolCardActions) *gtk.Box {
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

	weightText := fmt.Sprintf(t.SpoolWeightFmt, remaining, spool.TotalWeightG, percent)
	progress := widgets.NewLevelProgress(percent, weightText)
	progress.SetMarginStart(14)
	progress.SetMarginEnd(14)
	card.Append(progress)

	weight := gtk.NewLabel(weightText)
	weight.AddCSSClass("caption")
	weight.AddCSSClass("dim-label")
	weight.SetHAlign(gtk.AlignStart)
	weight.SetMarginStart(14)
	weight.SetMarginEnd(14)
	card.Append(weight)
	card.Append(newSpoolDryingSection(t, dryingStatus, dryingErr))

	actions := widgets.NewActions(8, gtk.AlignEnd)
	actions.SetMarginBottom(12)
	actions.SetMarginStart(14)
	actions.SetMarginEnd(14)
	dryBtn := spoolActionButton("weather-windy-symbolic", t.DrySpool)
	if dryingStatus.NeedsDrying {
		dryBtn.AddCSSClass("warning")
	}
	dryBtn.ConnectClicked(handlers.dry)
	actions.Append(dryBtn)
	editBtn := spoolActionButton("document-edit-symbolic", t.Edit)
	editBtn.ConnectClicked(handlers.edit)
	actions.Append(editBtn)
	archiveBtn := spoolActionButton("edit-delete-symbolic", t.Archive)
	archiveBtn.AddCSSClass("destructive-action")
	archiveBtn.ConnectClicked(handlers.archive)
	actions.Append(archiveBtn)
	card.Append(actions)

	return card
}

func spoolActionButton(iconName, tooltip string) *gtk.Button {
	button := gtk.NewButtonFromIconName(iconName)
	button.AddCSSClass("circular")
	button.SetTooltipText(tooltip)
	return button
}
