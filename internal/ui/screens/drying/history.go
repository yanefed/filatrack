package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
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
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginTop(4)
	card.SetMarginBottom(4)
	card.SetMarginStart(4)
	card.SetMarginEnd(4)

	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.SetMarginTop(12)
	row.SetMarginBottom(12)
	row.SetMarginStart(12)
	row.SetMarginEnd(12)
	row.Append(widgets.NewSpoolPreview(item.spool.ColorHex))

	identity := gtk.NewBox(gtk.OrientationVertical, 3)
	identity.SetHExpand(true)
	nameText := item.spool.Manufacturer
	if item.spool.DisplayCode != "" {
		nameText = fmt.Sprintf("%s — %s", item.spool.DisplayCode, item.spool.Manufacturer)
	}
	name := gtk.NewLabel(nameText)
	name.AddCSSClass("heading")
	name.SetHAlign(gtk.AlignStart)
	name.SetWrap(true)
	identity.Append(name)
	details := gtk.NewLabel(fmt.Sprintf("%s · %s", item.spool.Color, item.spool.Material))
	details.AddCSSClass("dim-label")
	details.SetHAlign(gtk.AlignStart)
	identity.Append(details)
	row.Append(identity)

	result := gtk.NewBox(gtk.OrientationVertical, 3)
	result.SetVAlign(gtk.AlignCenter)
	parameters := gtk.NewLabel(fmt.Sprintf("%d °C · %s", item.session.TemperatureC, formatHours(item.session.DurationHours, t.Hours)))
	parameters.AddCSSClass("heading")
	parameters.SetHAlign(gtk.AlignEnd)
	result.Append(parameters)
	date := gtk.NewLabel(item.session.DriedAt.Format("02.01.2006 15:04"))
	date.AddCSSClass("caption")
	date.AddCSSClass("dim-label")
	date.SetHAlign(gtk.AlignEnd)
	result.Append(date)
	row.Append(result)

	card.Append(row)
	return card
}
