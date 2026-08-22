package widgets

import (
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	levelProgressWidth  = 100
	levelProgressHeight = 8
)

// NewLevelProgress создаёт индикатор уровня, цвет которого плавно меняется
// от красного через жёлтый к зелёному.
func NewLevelProgress(percent float64, tooltip string) *gtk.DrawingArea {
	fraction := min(max(percent/100, 0), 1)
	red, green, blue := levelColor(fraction)

	area := gtk.NewDrawingArea()
	area.SetContentWidth(levelProgressWidth)
	area.SetContentHeight(levelProgressHeight)
	area.SetHExpand(true)
	area.SetTooltipText(tooltip)
	area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		barWidth := float64(width)
		barHeight := float64(height)
		radius := barHeight / 2
		centerY := radius
		endX := max(barWidth-radius, radius)

		cr.SetLineCap(cairo.LineCapRound)
		cr.SetLineWidth(barHeight)
		cr.SetSourceRGBA(0.5, 0.5, 0.5, 0.25)
		cr.MoveTo(radius, centerY)
		cr.LineTo(endX, centerY)
		cr.Stroke()

		if fraction <= 0 {
			return
		}
		fillEndX := radius + (endX-radius)*fraction
		cr.SetSourceRGBA(red, green, blue, 1)
		cr.MoveTo(radius, centerY)
		cr.LineTo(fillEndX, centerY)
		cr.Stroke()
	})

	return area
}

func levelColor(fraction float64) (red, green, blue float64) {
	fraction = min(max(fraction, 0), 1)
	redColor := [3]float64{0.88, 0.11, 0.14}
	yellowColor := [3]float64{0.96, 0.76, 0.07}
	greenColor := [3]float64{0.18, 0.76, 0.49}

	if fraction <= 0.5 {
		return interpolateColor(redColor, yellowColor, fraction*2)
	}
	return interpolateColor(yellowColor, greenColor, (fraction-0.5)*2)
}

func interpolateColor(from, to [3]float64, position float64) (red, green, blue float64) {
	return from[0] + (to[0]-from[0])*position,
		from[1] + (to[1]-from[1])*position,
		from[2] + (to[2]-from[2])*position
}
