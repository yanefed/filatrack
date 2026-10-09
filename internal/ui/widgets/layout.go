package widgets

import "awesomeProject/internal/ui/layout"
import "github.com/diamondburned/gotk4/pkg/gtk/v4"

const (
	screenSpacing = 12
	screenMargin  = 24
)

// NewScreen creates the common vertical shell used by application screens.
func NewScreen(title string) *gtk.Box {
	view := layout.Load("screen-shell")

	box := layout.Get[*gtk.Box](view, "box")

	_ = layout.Configure[*gtk.Label](view, "object_1637", func(w *gtk.Label) { w.SetLabel(title) })
	return box
}

// NewTitle creates a primary screen title.
func NewTitle(text string) *gtk.Label {
	view := layout.Load("screen-title")

	label := layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(text) })

	return label
}

// NewSectionTitle creates a left-aligned section heading.
func NewSectionTitle(text string) *gtk.Label {
	view := layout.Load("section-title")

	label := layout.Configure[*gtk.Label](view, "label", func(w *gtk.Label) { w.SetLabel(text) })

	return label
}

// NewMessageLabel creates a hidden, wrapping label for screen-level feedback.
func NewMessageLabel() *gtk.Label {
	view := layout.Load("message-label")

	label := layout.Get[*gtk.Label](view, "label")

	return label
}

// NewScrolled creates a vertically expanding scrolled container.
func NewScrolled(child gtk.Widgetter) *gtk.ScrolledWindow {
	view := layout.Load("scrolled-container")

	scrolled := layout.Get[*gtk.ScrolledWindow](view, "scrolled")

	layout.Mount(view, "scrolled_slot_1", child)
	return scrolled
}

// NewDialogContent creates the common padded content box used by modal windows.
func NewDialogContent(spacing int) *gtk.Box {
	view := layout.Load("dialog-content")

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { w.SetSpacing(spacing) })

	return box
}

// NewActions creates a horizontal button row.
func NewActions(spacing int, align gtk.Align) *gtk.Box {
	view := layout.Load("dialog-actions")

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { w.SetSpacing(spacing) })
	box.SetHAlign(align)
	return box
}

// ClearListBox removes every row from a list box.
func ClearListBox(list *gtk.ListBox) {
	for {
		row := list.RowAtIndex(0)
		if row == nil {
			return
		}
		list.Remove(row)
	}
}

// ReplaceBoxChild replaces all children of box with child.
func ReplaceBoxChild(box *gtk.Box, child gtk.Widgetter) {
	for current := box.FirstChild(); current != nil; current = box.FirstChild() {
		box.Remove(current)
	}
	box.Append(child)
}
