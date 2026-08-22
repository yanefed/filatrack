package widgets

import (
	"awesomeProject/internal/ui/colors"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ColorPicker объединяет подпись и нативную кнопку выбора цвета.
type ColorPicker struct {
	root   *gtk.Box
	button *gtk.ColorDialogButton
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

	return &ColorPicker{root: root, button: button}
}

// Widget возвращает корневой GTK-виджет выбора цвета.
func (picker *ColorPicker) Widget() *gtk.Box {
	return picker.root
}

// Hex возвращает выбранный цвет в нормализованном HEX-формате.
func (picker *ColorPicker) Hex() string {
	return colors.ToHex(picker.button.RGBA())
}

// SetHex устанавливает цвет из HEX-строки.
func (picker *ColorPicker) SetHex(value string) {
	picker.button.SetRGBA(colors.FromHex(value))
}
