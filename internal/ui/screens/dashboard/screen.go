package dashboard

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
	"github.com/yanefed/filatrack/internal/ui/widgets"
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
	metricsView := layout.Load("dashboard-metrics")
	widgets.ReplaceBoxChild(metricsSlot, newMetricsGrid(t, dashboardData{}, metricsView))
	var previous dashboardData
	initialized := false
	var refreshDashboard func()
	feedback := shared.NewReadFeedback(box, messageLabel, func() { refreshDashboard() })
	render := func(data dashboardData) {
		if initialized {
			if data.statsErr != nil {
				data.stats = previous.stats
			}
			if data.spoolsErr != nil && data.spools == nil {
				data.spools = previous.spools
			}
			if data.printersErr != nil {
				data.printers = previous.printers
			}
			if data.recentErr != nil {
				data.recent = previous.recent
			}
		}

		if !initialized || !reflect.DeepEqual(previous.stats, data.stats) || !reflect.DeepEqual(previous.spools, data.spools) {
			newMetricsGrid(t, data, metricsView)
		}
		updateActive(data.activePrinters())
		if !initialized || !reflect.DeepEqual(previous.recent, data.recent) {
			widgets.ReplaceBoxChild(recentSlot, newRecentPrintsPanel(t, data.recent))
		}
		if !initialized || !reflect.DeepEqual(previous.spools, data.spools) {
			widgets.ReplaceBoxChild(attentionSlot, newAttentionPanel(t, data.dryingWarnings()))
		}

		showDashboardErrors(t, messageLabel, data)
		if messageLabel.Visible() {
			feedback.FailureMessage(messageLabel.Label())
		} else {
			feedback.Success()
		}
		previous = data
		initialized = true
	}

	loader := shared.NewLoader[dashboardData](ctx, box)
	fullPending := false
	refreshDashboard = func() {
		feedback.Begin()
		fullPending = true
		loader.Request(func(ctx context.Context) dashboardData { return loadDashboardData(ctx, spoolSvc, printSvc, printerSvc) }, func(data dashboardData) { fullPending = false; render(data) })
	}
	refreshDashboardLive := func() {
		if fullPending {
			return
		}
		loader.Poll(func(ctx context.Context) dashboardData {
			printers, err := printerSvc.List(ctx)
			return dashboardData{printers: printers, printersErr: err}
		}, func(data dashboardData) {
			if data.printersErr == nil {
				updateActive(data.activePrinters())
			}
		})
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
