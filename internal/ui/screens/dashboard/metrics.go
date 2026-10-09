package dashboard

import (
	"fmt"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	metricCardMinWidth = 150
	metricGridSpacing  = 6
)

func newMetricsGrid(t i18n.T, data dashboardData) *gtk.FlowBox {
	view := layout.Load("dashboard-metrics")

	totalPrints := "—"
	successRate := "—"
	successDetail := t.StatsUnavailable
	totalUsed := "—"
	averageUsed := t.StatsUnavailable
	totalCost := "—"
	costDetail := t.StatsUnavailable
	if data.stats != nil {
		totalCost = fmt.Sprintf("%.2f %s", data.stats.TotalCost, data.stats.CostCurrency)
		if data.stats.EstimatedCostCount > 0 {
			totalCost = "≈" + totalCost
		}
		costDetail = t.PlasticCost
		if data.stats.UnpricedCount > 0 {
			costDetail += " · " + fmt.Sprintf(t.UnpricedPrintsFmt, data.stats.UnpricedCount)
		}
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

	_ = []*gtk.Box{
		newMetricCard(t.TotalPrints, totalPrints, "", layout.Scope(view, "total_prints")),
		newMetricCard(t.SuccessRate, successRate, successDetail, layout.Scope(view, "success_rate")),
		newMetricCard(t.Spools, fmt.Sprintf("%d", len(data.spools)), fmt.Sprintf(t.RemainingFilamentFmt, formatFilamentWeight(data.totalRemainingG())), layout.Scope(view, "spools")),
		newMetricCard(t.TotalUsed, totalUsed, averageUsed, layout.Scope(view, "total_used")),
		newMetricCard(t.TotalCost, totalCost, costDetail, layout.Scope(view, "total_cost")),
	}
	grid := layout.Get[*gtk.FlowBox](view, "grid")

	return grid
}

func newMetricCard(title, value, subtitle string, views ...*layout.View) *gtk.Box {
	view := layout.Use("dashboard-metric-card", views...)

	card := layout.Get[*gtk.Box](view, "card")

	label := layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(title) })

	label.SetTooltipText(title)

	valueLabel := layout.Configure[*gtk.Label](view, "valueLabel", func(w *gtk.Label) { w.SetLabel(value) })

	valueLabel.SetTooltipText(value)

	detail := layout.Get[*gtk.Label](view, "detail")
	detail.SetVisible(subtitle != "")
	detail.SetLabel(subtitle)
	detail.SetTooltipText(subtitle)

	return card
}

func formatFilamentWeight(weightG float64) string {
	if weightG >= 1000 {
		return fmt.Sprintf("%.2f kg", weightG/1000)
	}
	return fmt.Sprintf("%.0f g", weightG)
}
