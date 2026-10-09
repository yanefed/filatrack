package shared

import "github.com/yanefed/filatrack/internal/ui/layout"
import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// NewModalWindow creates a window that blocks and stays above its parent.
func NewModalWindow(parent *gtk.Window) *gtk.Window {
	view := layout.Load("modal-window")

	dialog := layout.Get[*gtk.Window](view, "dialog")

	if parent != nil {
		dialog.SetTransientFor(parent)
		dialog.SetDestroyWithParent(true)
	}
	return dialog
}
