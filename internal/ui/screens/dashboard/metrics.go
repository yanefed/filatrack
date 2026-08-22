package dashboard

import (
	"awesomeProject/internal/i18n"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

const (
	metricCardMinWidth = 150
	metricGridSpacing  = 6
)

func newMetricsGrid(t i18n.T, data dashboardData) *gtk.FlowBox {
	totalPrints := "—"
	successRate := "—"
	successDetail := t.StatsUnavailable
	totalUsed := "—"
	averageUsed := t.StatsUnavailable
	if data.stats != nil {
		totalPrints = fmt.Sprintf("%d", data.stats.TotalPrints)
		rate := 0.0
		if data.stats.TotalPrints > 0 {
			rate = float64(data.stats.SuccessCount) / float64(data.stats.TotalPrints) * 100
		}
		successRate = fmt.Sprintf("%.0f%%", rate)
		successDetail = fmt.Sprintf("%d / %d", data.stats.SuccessCount, data.stats.TotalPrints)
		totalUsed = formatFilamentWeight(data.stats.TotalWeightG)
		averageUsed = fmt.Sprintf("%s: %s", t.AvgPerPrint, formatFilamentWeight(data.stats.AvgWeightG))
	}

	cards := []*gtk.Box{
		newMetricCard("printer-symbolic", "accent", t.TotalPrints, totalPrints, ""),
		newMetricCard("object-select-symbolic", "success", t.SuccessRate, successRate, successDetail),
		newMetricCard("media-record-symbolic", "warning", t.TotalUsed, totalUsed, averageUsed),
		newMetricCard("view-grid-symbolic", "accent", t.Spools, fmt.Sprintf("%d", len(data.spools)), fmt.Sprintf(t.RemainingFilamentFmt, formatFilamentWeight(data.totalRemainingG()))),
	}
	grid := gtk.NewFlowBox()
	grid.SetSelectionMode(gtk.SelectionNone)
	grid.SetHomogeneous(true)
	grid.SetMinChildrenPerLine(1)
	grid.SetMaxChildrenPerLine(4)
	grid.SetColumnSpacing(metricGridSpacing)
	grid.SetRowSpacing(metricGridSpacing)
	for _, card := range cards {
		grid.Append(card)
	}
	return grid
}

func newMetricCard(iconName, colorClass, title, value, subtitle string) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 5)
	card.AddCSSClass("card")
	card.SetSizeRequest(metricCardMinWidth, 112)
	card.SetMarginTop(2)
	card.SetMarginBottom(2)
	card.SetMarginStart(2)
	card.SetMarginEnd(2)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(12)
	header.SetMarginStart(12)
	header.SetMarginEnd(12)
	icon := gtk.NewImageFromIconName(iconName)
	icon.AddCSSClass(colorClass)
	header.Append(icon)
	label := gtk.NewLabel(title)
	label.AddCSSClass("dim-label")
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	label.SetEllipsize(pango.EllipsizeEnd)
	header.Append(label)
	card.Append(header)

	valueLabel := gtk.NewLabel(value)
	valueLabel.AddCSSClass("title-1")
	valueLabel.SetHAlign(gtk.AlignStart)
	valueLabel.SetMarginStart(12)
	valueLabel.SetMarginEnd(12)
	card.Append(valueLabel)

	if subtitle != "" {
		detail := gtk.NewLabel(subtitle)
		detail.AddCSSClass("caption")
		detail.AddCSSClass("dim-label")
		detail.SetHAlign(gtk.AlignStart)
		detail.SetEllipsize(pango.EllipsizeEnd)
		detail.SetMarginBottom(10)
		detail.SetMarginStart(12)
		detail.SetMarginEnd(12)
		card.Append(detail)
	}
	return card
}

func formatFilamentWeight(weightG float64) string {
	if weightG >= 1000 {
		return fmt.Sprintf("%.2f kg", weightG/1000)
	}
	return fmt.Sprintf("%.0f g", weightG)
}
