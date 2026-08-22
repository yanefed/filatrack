package shared

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// NewModalWindow creates a window that blocks and stays above its parent.
func NewModalWindow(parent *gtk.Window) *gtk.Window {
	dialog := gtk.NewWindow()
	dialog.SetModal(true)
	if parent != nil {
		dialog.SetTransientFor(parent)
		dialog.SetDestroyWithParent(true)
	}
	return dialog
}
