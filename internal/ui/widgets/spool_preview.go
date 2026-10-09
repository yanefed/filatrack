package widgets

import (
	"github.com/yanefed/filatrack/internal/ui/colors"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSpoolPreview создаёт схематичное изображение катушки с филаментом
// указанного цвета.
func NewSpoolPreview(colorHex string, views ...*layout.View) *gtk.DrawingArea {
	view := layout.Use("spool-preview", views...)

	const size = 58

	color := colors.FromHex(colorHex)
	area := layout.Get[*gtk.DrawingArea](view, "area")

	area.SetTooltipText(colors.ToHex(color))
	area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		centerX := float64(width) / 2
		centerY := float64(height) / 2
		radius := math.Min(float64(width), float64(height))/2 - 2

		drawSpoolFlange(cr, centerX, centerY, radius)
		drawFilament(cr, centerX, centerY, radius, color.Red(), color.Green(), color.Blue())
		drawSpoolHub(cr, centerX, centerY, radius)
	})

	return area
}

func drawSpoolFlange(cr *cairo.Context, centerX, centerY, radius float64) {
	cr.Arc(centerX, centerY, radius, 0, 2*math.Pi)
	cr.SetSourceRGBA(0.32, 0.32, 0.32, 1)
	cr.FillPreserve()
	cr.SetSourceRGBA(0.18, 0.18, 0.18, 1)
	cr.SetLineWidth(1.5)
	cr.Stroke()
}

func drawFilament(cr *cairo.Context, centerX, centerY, radius float64, red, green, blue float32) {
	cr.Arc(centerX, centerY, radius*0.78, 0, 2*math.Pi)
	cr.SetSourceRGBA(float64(red), float64(green), float64(blue), 1)
	cr.Fill()

	cr.SetSourceRGBA(0.12, 0.12, 0.12, 0.3)
	cr.SetLineWidth(1)
	for factor := 0.45; factor <= 0.75; factor += 0.15 {
		cr.Arc(centerX, centerY, radius*factor, 0, 2*math.Pi)
		cr.Stroke()
	}
}

func drawSpoolHub(cr *cairo.Context, centerX, centerY, radius float64) {
	cr.Arc(centerX, centerY, radius*0.31, 0, 2*math.Pi)
	cr.SetSourceRGBA(0.38, 0.38, 0.38, 1)
	cr.Fill()
	cr.Arc(centerX, centerY, radius*0.14, 0, 2*math.Pi)
	cr.SetSourceRGBA(0.12, 0.12, 0.12, 1)
	cr.Fill()
}
