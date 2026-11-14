package drying

import (
	"context"
	"fmt"
	"reflect"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/shared"
)

func newRecommendationsView(t i18n.T, ctx context.Context, dryingSvc *service.DryingService, views ...*layout.View) (*gtk.ScrolledWindow, func()) {
	view := layout.Use("drying-recommendations", views...)

	row := layout.Get[*gtk.Box](view, "row")

	scrolled := layout.Get[*gtk.ScrolledWindow](view, "scrolled")

	var previous []domain.DryingProfile
	type result struct {
		profiles []domain.DryingProfile
		err      error
	}
	loader := shared.NewLoader[result](ctx, scrolled)
	refresh := func() {
		loader.Request(func(ctx context.Context) result { p, e := dryingSvc.ListRecommendations(ctx); return result{p, e} }, func(data result) {
			if data.err != nil || reflect.DeepEqual(previous, data.profiles) {
				return
			}
			previous = data.profiles
			for child := row.FirstChild(); child != nil; child = row.FirstChild() {
				row.Remove(child)
			}
			for _, profile := range data.profiles {
				row.Append(newRecommendationCard(t, profile))
			}
		})
	}

	refresh()
	return scrolled, refresh
}

func newRecommendationCard(t i18n.T, profile domain.DryingProfile) *gtk.Box {
	view := layout.Load("drying-recommendation-card")

	card := layout.Get[*gtk.Box](view, "card")

	_ = layout.Configure[*gtk.Label](view, "material", func(w *gtk.Label) { w.SetLabel(string(profile.Material)) })
	_ = layout.Configure[*gtk.Label](view, "parameters", func(w *gtk.Label) {
		w.SetLabel(fmt.Sprintf("%d °C · %s", profile.TemperatureC, formatHours(profile.DurationHours, t.Hours)))
	})
	_ = layout.Configure[*gtk.Label](view, "interval", func(w *gtk.Label) { w.SetLabel(fmt.Sprintf("%s: %d %s", t.MaxDaysNoDry, profile.MaxDaysNoDry, t.Days)) })

	return card
}

func formatHours(hours float64, unit string) string {
	return fmt.Sprintf("%g %s", hours, unit)
}
