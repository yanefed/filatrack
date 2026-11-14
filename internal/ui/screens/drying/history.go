package drying

import (
	"context"
	"fmt"
	"sort"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

type dryingHistoryItem struct {
	spool   domain.Spool
	session domain.DryingSession
}

func loadDryingHistory(ctx context.Context, dryingSvc *service.DryingService, spoolSvc *service.SpoolService) ([]dryingHistoryItem, error) {
	notArchived := false
	spools, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &notArchived})
	if err != nil {
		return nil, err
	}
	items := make([]dryingHistoryItem, 0)
	byID := make(map[string]domain.Spool, len(spools))
	for _, spool := range spools {
		byID[spool.ID] = spool
	}
	sessions, err := dryingSvc.ListHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading drying history: %w", err)
	}
	for _, session := range sessions {
		if spool, ok := byID[session.SpoolID]; ok {
			items = append(items, dryingHistoryItem{spool: spool, session: session})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].session.DriedAt.After(items[j].session.DriedAt)
	})
	return items, nil
}

func newDryingHistoryCard(t i18n.T, item dryingHistoryItem, views ...*layout.View) *gtk.Box {
	view := layout.Use("drying-history-card", views...)

	card := layout.Get[*gtk.Box](view, "card")

	_ = widgets.NewSpoolPreview(item.spool.ColorHex, layout.Scope(view, "row_slot_1"))

	nameText := item.spool.Manufacturer
	if item.spool.DisplayCode != "" {
		nameText = fmt.Sprintf("%s — %s", item.spool.DisplayCode, item.spool.Manufacturer)
	}
	_ = layout.Configure[*gtk.Label](view, "name", func(w *gtk.Label) { w.SetLabel(nameText) })
	_ = layout.Configure[*gtk.Label](view, "details", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s · %s", item.spool.Color, item.spool.Material)) })
	_ = layout.Configure[*gtk.Label](view, "parameters", func(w *gtk.Label) {
		w.SetLabel(fmt.Sprintf("%d °C · %s", item.session.TemperatureC, formatHours(item.session.DurationHours, t.Hours)))
	})
	_ = layout.Configure[*gtk.Label](view, "date", func(w *gtk.Label) { w.SetLabel(item.session.DriedAt.Format("02.01.2006 15:04")) })

	return card
}
