package dashboard

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDashboard создаёт экран сводной статистики и предупреждений о сушке.
func NewDashboard(ctx context.Context, spoolSvc *service.SpoolService, printSvc *service.PrintService) (*gtk.Box, func()) {
	t := i18n.L()

	box := widgets.NewScreen(t.Dashboard)

	statsLabel := gtk.NewLabel("...")
	statsLabel.SetHAlign(gtk.AlignStart)
	box.Append(statsLabel)

	warningsLabel := gtk.NewLabel("")
	warningsLabel.SetHAlign(gtk.AlignStart)
	warningsLabel.SetWrap(true)
	box.Append(warningsLabel)

	refreshDashboard := func() {
		stats, err := printSvc.GetStats(ctx, repository.StatsFilter{})
		if err != nil {
			statsLabel.SetText(t.StatsUnavailable)
			return
		}

		successRate := 0.0
		if stats.TotalPrints > 0 {
			successRate = float64(stats.SuccessCount) / float64(stats.TotalPrints) * 100
		}

		statsLabel.SetText(fmt.Sprintf(
			"%s: %d\n%s: %d (%.0f%%)\n%s: %.1f g\n%s: %.1f g",
			t.TotalPrints, stats.TotalPrints,
			t.SuccessRate, stats.SuccessCount, successRate,
			t.TotalUsed, stats.TotalWeightG,
			t.AvgPerPrint, stats.AvgWeightG,
		))

		// Проверка сушки
		notArchived := false
		spools, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
		if err != nil {
			return
		}

		var warnings []string
		for i := range spools {
			needs, err := spoolSvc.NeedsDrying(ctx, &spools[i])
			if err != nil {
				continue
			}
			if needs {
				warnings = append(warnings, fmt.Sprintf("⚠ %s %s (%s) — %s",
					spools[i].Manufacturer, spools[i].Color, string(spools[i].Material),
					t.SpoolNeedsDrying))
			}
		}

		if len(warnings) > 0 {
			text := t.DryingWarnings + ":\n"
			for _, w := range warnings {
				text += w + "\n"
			}
			warningsLabel.SetText(text)
		} else {
			warningsLabel.SetText(t.NoDryingWarnings)
		}
	}

	refreshDashboard()

	return box, refreshDashboard
}
