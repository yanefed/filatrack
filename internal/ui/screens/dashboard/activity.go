package dashboard

import (
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/widgets"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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
			view := layout.Load("dashboard-warning-separator")

			card.Append(layout.Get[*gtk.Separator](view, "object_191"))
		}
		card.Append(newDryingWarningRow(t, warnings[i]))
	}
	if hidden := len(warnings) - visible; hidden > 0 {
		view := layout.Load("dashboard-more-warnings")

		card.Append(layout.Get[*gtk.Separator](view, "object_193"))
		more := layout.Configure[*gtk.Label](view, "more", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf(t.MoreItemsFmt, hidden)) })

		card.Append(more)
	}
	return card
}

func newDryingWarningRow(t i18n.T, spool dashboardSpool) *gtk.Box {
	view := layout.Load("dashboard-warning-row")

	row := layout.Get[*gtk.Box](view, "row")

	_ = widgets.NewSpoolPreview(spool.ColorHex, layout.Scope(view, "row_slot_1"))

	name := spool.Manufacturer
	if spool.DisplayCode != "" {
		name = fmt.Sprintf("%s — %s", spool.DisplayCode, spool.Manufacturer)
	}
	_ = layout.Configure[*gtk.Label](view, "title", func(w *gtk.Label) { w.SetLabel(name) })
	_ = layout.Configure[*gtk.Label](view, "details", func(w *gtk.Label) {
		w.SetLabel(fmt.Sprintf("%s · %s · %s", spool.Color, spool.Material, t.SpoolNeedsDrying))
	})

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
			view := layout.Load("dashboard-print-separator")

			card.Append(layout.Get[*gtk.Separator](view, "object_225"))
		}
		card.Append(newRecentPrintRow(job))
	}
	return card
}

func newRecentPrintRow(job domain.PrintJob) *gtk.Box {
	view := layout.Load("dashboard-print-row")

	row := layout.Get[*gtk.Box](view, "row")

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
	icon := layout.Configure[*gtk.Image](view, "icon", func(w *gtk.Image) { w.SetFromIconName(statusIcon) })

	icon.AddCSSClass(statusClass)

	name := job.ModelName
	if name == "" {
		name = job.GcodeFilename
	}
	_ = layout.Configure[*gtk.Label](view, "title", func(w *gtk.Label) { w.SetLabel(name) })

	weight := job.ActualWeightG
	if weight == 0 {
		weight = job.EstimatedWeightG
	}
	_ = layout.Configure[*gtk.Label](view, "details", func(w *gtk.Label) {
		w.SetLabel(fmt.Sprintf("%s · %s · %s", job.Status, formatFilamentWeight(weight), job.PrintedAt.Format("02.01.2006")))
	})

	return row
}

func newDashboardPanelCard() *gtk.Box {
	view := layout.Load("dashboard-panel")

	card := layout.Get[*gtk.Box](view, "card")

	return card
}

func newDashboardPanelState(iconName, colorClass, text string) *gtk.Box {
	view := layout.Load("dashboard-panel-state")

	row := layout.Get[*gtk.Box](view, "row")

	icon := layout.Configure[*gtk.Image](view, "icon", func(w *gtk.Image) { w.SetFromIconName(iconName) })
	icon.AddCSSClass(colorClass)
	_ = layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(text) })

	return row
}
