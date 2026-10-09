package widgets

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

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
