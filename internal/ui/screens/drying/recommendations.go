package drying

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	recommendationCardWidth      = 190
	recommendationCardHeight     = 128
	recommendationViewportHeight = 140
	recommendationStripHeight    = 158
)

var recommendedMaterials = []domain.Material{
	domain.MaterialPLA,
	domain.MaterialPETG,
	domain.MaterialABS,
	domain.MaterialASA,
	domain.MaterialTPU,
	domain.MaterialNylon,
	domain.MaterialPC,
}

func newRecommendationsView(t i18n.T, ctx context.Context, dryingSvc *service.DryingService) (*gtk.ScrolledWindow, func()) {
	row := gtk.NewBox(gtk.OrientationHorizontal, 10)
	row.SetMarginBottom(8)

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	scrolled.SetOverlayScrolling(false)
	scrolled.SetMinContentHeight(recommendationViewportHeight)
	scrolled.SetSizeRequest(-1, recommendationStripHeight)
	scrolled.SetChild(row)

	refresh := func() {
		for child := row.FirstChild(); child != nil; child = row.FirstChild() {
			row.Remove(child)
		}
		for _, material := range recommendedMaterials {
			profile := dryingSvc.GetRecommendation(ctx, material)
			if profile != nil {
				row.Append(newRecommendationCard(t, *profile))
			}
		}
	}
	refresh()
	return scrolled, refresh
}

func newRecommendationCard(t i18n.T, profile domain.DryingProfile) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 5)
	card.AddCSSClass("card")
	card.SetSizeRequest(recommendationCardWidth, recommendationCardHeight)
	card.SetMarginTop(2)
	card.SetMarginBottom(2)
	card.SetMarginStart(2)
	card.SetMarginEnd(2)

	material := gtk.NewLabel(string(profile.Material))
	material.AddCSSClass("heading")
	material.SetHAlign(gtk.AlignStart)
	material.SetMarginTop(12)
	material.SetMarginStart(12)
	material.SetMarginEnd(12)
	card.Append(material)

	parameters := gtk.NewLabel(fmt.Sprintf("%d °C · %s", profile.TemperatureC, formatHours(profile.DurationHours, t.Hours)))
	parameters.AddCSSClass("title-3")
	parameters.SetHAlign(gtk.AlignStart)
	parameters.SetMarginStart(12)
	parameters.SetMarginEnd(12)
	card.Append(parameters)

	interval := gtk.NewLabel(fmt.Sprintf("%s: %d %s", t.MaxDaysNoDry, profile.MaxDaysNoDry, t.Days))
	interval.AddCSSClass("caption")
	interval.AddCSSClass("dim-label")
	interval.SetHAlign(gtk.AlignStart)
	interval.SetWrap(true)
	interval.SetMarginBottom(12)
	interval.SetMarginStart(12)
	interval.SetMarginEnd(12)
	card.Append(interval)
	return card
}

func formatHours(hours float64, unit string) string {
	return fmt.Sprintf("%g %s", hours, unit)
}
