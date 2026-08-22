// Package colors содержит преобразования цветов, используемые UI.
package colors

import (
	"fmt"
	"math"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

const DefaultHex = "#000000"

// FromHex преобразует HEX-строку в цвет GDK. Пустое или некорректное значение
// возвращает цвет по умолчанию.
func FromHex(value string) *gdk.RGBA {
	color := gdk.NewRGBA(0, 0, 0, 1)
	if value != "" {
		color.Parse(value)
	}
	return &color
}

// ToHex возвращает непрозрачный цвет в формате #rrggbb.
func ToHex(color *gdk.RGBA) string {
	if color == nil {
		return DefaultHex
	}
	channel := func(value float32) int {
		return int(math.Round(float64(min(max(value, 0), 1) * 255)))
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(color.Red()), channel(color.Green()), channel(color.Blue()))
}
