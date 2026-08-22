package dashboard

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/widgets"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

const dashboardWarningLimit = 3

func newAttentionPanel(t i18n.T, warnings []dashboardSpool) *gtk.Box {
	card := newDashboardPanelCard()
	if len(warnings) == 0 {
		card.Append(newDashboardPanelState("object-select-symbolic", "success", t.NoDryingWarnings))
		return card
	}
	visible := min(len(warnings), dashboardWarningLimit)
	for i := range visible {
		if i > 0 {
			card.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		}
		card.Append(newDryingWarningRow(t, warnings[i]))
	}
	if hidden := len(warnings) - visible; hidden > 0 {
		card.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		more := gtk.NewLabel(fmt.Sprintf(t.MoreItemsFmt, hidden))
		more.AddCSSClass("caption")
		more.AddCSSClass("dim-label")
		more.SetHAlign(gtk.AlignStart)
		more.SetMarginTop(10)
		more.SetMarginBottom(10)
		more.SetMarginStart(12)
		more.SetMarginEnd(12)
		card.Append(more)
	}
	return card
}

func newDryingWarningRow(t i18n.T, spool dashboardSpool) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 10)
	row.SetMarginTop(10)
	row.SetMarginBottom(10)
	row.SetMarginStart(12)
	row.SetMarginEnd(12)
	preview := widgets.NewSpoolPreview(spool.ColorHex)
	preview.SetSizeRequest(48, 48)
	row.Append(preview)

	labels := gtk.NewBox(gtk.OrientationVertical, 2)
	labels.SetHExpand(true)
	name := spool.Manufacturer
	if spool.DisplayCode != "" {
		name = fmt.Sprintf("%s — %s", spool.DisplayCode, spool.Manufacturer)
	}
	title := gtk.NewLabel(name)
	title.AddCSSClass("heading")
	title.SetHAlign(gtk.AlignStart)
	title.SetEllipsize(pango.EllipsizeEnd)
	labels.Append(title)
	details := gtk.NewLabel(fmt.Sprintf("%s · %s · %s", spool.Color, spool.Material, t.SpoolNeedsDrying))
	details.AddCSSClass("caption")
	details.AddCSSClass("warning")
	details.SetHAlign(gtk.AlignStart)
	details.SetEllipsize(pango.EllipsizeEnd)
	labels.Append(details)
	row.Append(labels)
	return row
}

func newRecentPrintsPanel(t i18n.T, jobs []domain.PrintJob) *gtk.Box {
	card := newDashboardPanelCard()
	if len(jobs) == 0 {
		card.Append(newDashboardPanelState("document-open-recent-symbolic", "dim-label", t.NoPrintRecords))
		return card
	}
	for i, job := range jobs {
		if i > 0 {
			card.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		}
		card.Append(newRecentPrintRow(job))
	}
	return card
}

func newRecentPrintRow(job domain.PrintJob) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 10)
	row.SetMarginTop(10)
	row.SetMarginBottom(10)
	row.SetMarginStart(12)
	row.SetMarginEnd(12)

	statusIcon := "object-select-symbolic"
	statusClass := "success"
	switch job.Status {
	case domain.PrintStatusFailed:
		statusIcon = "dialog-error-symbolic"
		statusClass = "error"
	case domain.PrintStatusCancelled:
		statusIcon = "process-stop-symbolic"
		statusClass = "warning"
	}
	icon := gtk.NewImageFromIconName(statusIcon)
	icon.SetPixelSize(24)
	icon.AddCSSClass(statusClass)
	row.Append(icon)

	labels := gtk.NewBox(gtk.OrientationVertical, 2)
	labels.SetHExpand(true)
	name := job.ModelName
	if name == "" {
		name = job.GcodeFilename
	}
	title := gtk.NewLabel(name)
	title.AddCSSClass("heading")
	title.SetHAlign(gtk.AlignStart)
	title.SetEllipsize(pango.EllipsizeEnd)
	labels.Append(title)
	weight := job.ActualWeightG
	if weight == 0 {
		weight = job.EstimatedWeightG
	}
	details := gtk.NewLabel(fmt.Sprintf("%s · %s · %s", job.Status, formatFilamentWeight(weight), job.PrintedAt.Format("02.01.2006")))
	details.AddCSSClass("caption")
	details.AddCSSClass("dim-label")
	details.SetHAlign(gtk.AlignStart)
	details.SetEllipsize(pango.EllipsizeEnd)
	labels.Append(details)
	row.Append(labels)
	return row
}

func newDashboardPanelCard() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginTop(2)
	card.SetMarginBottom(2)
	card.SetMarginStart(2)
	card.SetMarginEnd(2)
	return card
}

func newDashboardPanelState(iconName, colorClass, text string) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 10)
	row.SetMarginTop(14)
	row.SetMarginBottom(14)
	row.SetMarginStart(14)
	row.SetMarginEnd(14)
	icon := gtk.NewImageFromIconName(iconName)
	icon.AddCSSClass(colorClass)
	row.Append(icon)
	label := gtk.NewLabel(text)
	label.AddCSSClass("dim-label")
	label.SetHAlign(gtk.AlignStart)
	label.SetWrap(true)
	row.Append(label)
	return row
}
