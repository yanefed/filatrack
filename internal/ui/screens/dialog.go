package screens

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// newModalWindow создаёт окно, которое блокирует родителя и остаётся над ним.
func newModalWindow(parent *gtk.Window) *gtk.Window {
	dialog := gtk.NewWindow()
	dialog.SetModal(true)
	if parent != nil {
		dialog.SetTransientFor(parent)
		dialog.SetDestroyWithParent(true)
	}
	return dialog
}
