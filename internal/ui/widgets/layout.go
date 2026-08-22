package widgets

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

const (
	screenSpacing = 12
	screenMargin  = 24
)

// NewScreen creates the common vertical shell used by application screens.
func NewScreen(title string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, screenSpacing)
	box.SetMarginTop(screenMargin)
	box.SetMarginBottom(screenMargin)
	box.SetMarginStart(screenMargin)
	box.SetMarginEnd(screenMargin)
	box.Append(NewTitle(title))
	return box
}

// NewTitle creates a primary screen title.
func NewTitle(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.AddCSSClass("title-1")
	return label
}

// NewSectionTitle creates a left-aligned section heading.
func NewSectionTitle(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("title-3")
	return label
}

// NewMessageLabel creates a hidden, wrapping label for screen-level feedback.
func NewMessageLabel() *gtk.Label {
	label := gtk.NewLabel("")
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("dim-label")
	label.SetWrap(true)
	label.SetVisible(false)
	return label
}

// NewScrolled creates a vertically expanding scrolled container.
func NewScrolled(child gtk.Widgetter) *gtk.ScrolledWindow {
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetVExpand(true)
	scrolled.SetChild(child)
	return scrolled
}

// NewDialogContent creates the common padded content box used by modal windows.
func NewDialogContent(spacing int) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, spacing)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)
	return box
}

// NewActions creates a horizontal button row.
func NewActions(spacing int, align gtk.Align) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, spacing)
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
