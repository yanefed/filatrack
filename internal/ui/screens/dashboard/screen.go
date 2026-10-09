package dashboard

import (
	"context"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/widgets"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDashboard creates the application overview and its lightweight live refresh.
func NewDashboard(ctx context.Context, spoolSvc *service.SpoolService, printSvc *service.PrintService, printerSvc *service.PrinterService, views ...*layout.View) (*gtk.Box, func(), func()) {
	view := layout.Use("dashboard-screen", views...)

	t := i18n.L()
	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(t.Dashboard) })

	messageLabel := layout.Get[*gtk.Label](view, "messageLabel")

	layout.Get[*gtk.Label](view, "active_title").SetLabel(t.ActivePrints)
	layout.Get[*gtk.Label](view, "recent_title").SetLabel(t.RecentPrints)
	layout.Get[*gtk.Label](view, "attention_title").SetLabel(t.DryingWarnings)

	activeSlot := layout.Get[*gtk.Box](view, "activeSlot")

	metricsSlot := layout.Get[*gtk.Box](view, "metrics_slot")
	recentSlot := layout.Get[*gtk.Box](view, "recent_slot")
	attentionSlot := layout.Get[*gtk.Box](view, "attention_slot")
	var activeIDs []string
	var activeStatuses []*widgets.PrinterStatusWidget
	updateActive := func(printers []domain.Printer) {
		unchanged := len(printers) == len(activeIDs)
		if unchanged {
			for i := range printers {
				if printers[i].ID != activeIDs[i] {
					unchanged = false
					break
				}
			}
		}
		if unchanged {
			for i := range printers {
				activeStatuses[i].Update(t, printers[i])
			}
			return
		}

		view, statuses := newActivePrintsView(t, printers)
		widgets.ReplaceBoxChild(activeSlot, view)
		activeIDs = make([]string, len(printers))
		for i := range printers {
			activeIDs[i] = printers[i].ID
		}
		activeStatuses = statuses
	}
	render := func(data dashboardData) {

		widgets.ReplaceBoxChild(metricsSlot, newMetricsGrid(t, data))
		updateActive(data.activePrinters())
		widgets.ReplaceBoxChild(recentSlot, newRecentPrintsPanel(t, data.recent))
		widgets.ReplaceBoxChild(attentionSlot, newAttentionPanel(t, data.dryingWarnings()))

		showDashboardErrors(t, messageLabel, data)
	}

	refreshDashboard := func() {
		render(loadDashboardData(ctx, spoolSvc, printSvc, printerSvc))
	}
	refreshDashboardLive := func() {
		printers, err := printerSvc.List(ctx)
		if err != nil {
			return
		}
		data := dashboardData{printers: printers}
		updateActive(data.activePrinters())
	}

	refreshDashboard()
	return box, refreshDashboard, refreshDashboardLive
}

func showDashboardErrors(t i18n.T, label *gtk.Label, data dashboardData) {
	messages := make([]string, 0, 4)
	if data.statsErr != nil {
		messages = append(messages, t.StatsUnavailable)
	}
	for _, err := range []error{data.spoolsErr, data.printersErr, data.recentErr} {
		if err != nil {
			messages = append(messages, fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
		}
	}
	label.SetVisible(len(messages) > 0)
	label.SetLabel(strings.Join(messages, " · "))
}
