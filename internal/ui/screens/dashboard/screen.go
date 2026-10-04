package dashboard

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewDashboard creates the application overview and its lightweight live refresh.
func NewDashboard(ctx context.Context, spoolSvc *service.SpoolService, printSvc *service.PrintService, printerSvc *service.PrinterService) (*gtk.Box, func(), func()) {
	t := i18n.L()
	box := widgets.NewScreen(t.Dashboard)

	messageLabel := widgets.NewMessageLabel()
	box.Append(messageLabel)
	content := gtk.NewBox(gtk.OrientationVertical, 18)
	box.Append(widgets.NewScrolled(content))

	activeSlot := gtk.NewBox(gtk.OrientationVertical, 0)
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
		for child := content.FirstChild(); child != nil; child = content.FirstChild() {
			content.Remove(child)
		}
		content.Append(newMetricsGrid(t, data))

		content.Append(widgets.NewSectionTitle(t.ActivePrints))
		updateActive(data.activePrinters())
		content.Append(activeSlot)

		columns := gtk.NewBox(gtk.OrientationHorizontal, 12)
		columns.SetHomogeneous(true)
		recent := gtk.NewBox(gtk.OrientationVertical, 8)
		recent.SetHExpand(true)
		recent.Append(widgets.NewSectionTitle(t.RecentPrints))
		recent.Append(newRecentPrintsPanel(t, data.recent))
		columns.Append(recent)

		attention := gtk.NewBox(gtk.OrientationVertical, 8)
		attention.SetHExpand(true)
		attention.Append(widgets.NewSectionTitle(t.DryingWarnings))
		attention.Append(newAttentionPanel(t, data.dryingWarnings()))
		columns.Append(attention)
		content.Append(columns)

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
