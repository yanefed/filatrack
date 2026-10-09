package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"sort"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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
	for _, spool := range spools {
		sessions, err := dryingSvc.ListForSpool(ctx, spool.ID)
		if err != nil {
			return nil, fmt.Errorf("loading drying history for spool %s: %w", spool.ID, err)
		}
		for _, session := range sessions {
			items = append(items, dryingHistoryItem{spool: spool, session: session})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].session.DriedAt.After(items[j].session.DriedAt)
	})
	return items, nil
}

func newDryingHistoryCard(t i18n.T, item dryingHistoryItem) *gtk.Box {
	view := layout.Load("drying-history-card")

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
