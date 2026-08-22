package widgets

import (
	"awesomeProject/internal/ui/colors"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ColorPicker объединяет подпись и нативную кнопку выбора цвета.
type ColorPicker struct {
	root         *gtk.Box
	button       *gtk.ColorDialogButton
	buttonInRoot bool
}

// NewColorPicker создаёт подписанный выбор непрозрачного цвета.
func NewColorPicker(title, initialHex string) *ColorPicker {
	dialog := gtk.NewColorDialog()
	dialog.SetTitle(title)
	dialog.SetModal(true)
	dialog.SetWithAlpha(false)

	button := gtk.NewColorDialogButton(dialog)
	button.SetTooltipText(title)
	button.SetRGBA(colors.FromHex(initialHex))

	root := gtk.NewBox(gtk.OrientationHorizontal, 8)
	label := gtk.NewLabel(title)
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	root.Append(label)
	root.Append(button)

	return &ColorPicker{root: root, button: button, buttonInRoot: true}
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
