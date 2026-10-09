package widgets

import (
	"github.com/yanefed/filatrack/internal/ui/colors"
	"github.com/yanefed/filatrack/internal/ui/layout"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ColorPicker объединяет подпись и нативную кнопку выбора цвета.
type ColorPicker struct {
	root         *gtk.Box
	button       *gtk.ColorDialogButton
	buttonInRoot bool
}

// NewColorPicker создаёт подписанный выбор непрозрачного цвета.
func NewColorPicker(title, initialHex string, views ...*layout.View) *ColorPicker {
	view := layout.Use("color-picker", views...)

	dialog := layout.Get[*gtk.ColorDialog](view, "dialog")
	dialog.SetTitle(title)

	button := layout.Configure[*gtk.ColorDialogButton](view, "button", func(w *gtk.ColorDialogButton) { w.SetDialog(dialog) })
	button.SetTooltipText(title)
	button.SetRGBA(colors.FromHex(initialHex))

	root := layout.Get[*gtk.Box](view, "root")
	_ = layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(title) })

	parent := button.Parent()
	inRoot := parent != nil && coreglib.BaseObject(parent).Native() == coreglib.BaseObject(root).Native()
	return &ColorPicker{root: root, button: button, buttonInRoot: inRoot}
}

// Widget возвращает корневой GTK-виджет выбора цвета.
func (picker *ColorPicker) Widget() *gtk.Box {
	if !picker.buttonInRoot && picker.button.Parent() == nil {
		picker.root.Append(picker.button)
		picker.buttonInRoot = true
	}
	return picker.root
}

// Button returns the compact native color selector for embedding in a row.
func (picker *ColorPicker) Button() *gtk.ColorDialogButton {
	if picker.buttonInRoot {
		picker.root.Remove(picker.button)
		picker.buttonInRoot = false
	}
	return picker.button
}

// Hex возвращает выбранный цвет в нормализованном HEX-формате.
func (picker *ColorPicker) Hex() string {
	return colors.ToHex(picker.button.RGBA())
}

// SetHex устанавливает цвет из HEX-строки.
func (picker *ColorPicker) SetHex(value string) {
	picker.button.SetRGBA(colors.FromHex(value))
}
