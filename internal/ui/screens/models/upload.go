package models

import (
	"awesomeProject/internal/i18n"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type modelUpload struct {
	pathEntry *gtk.Entry
	button    *gtk.Button
}

func newModelUpload(t i18n.T, parent *gtk.Window) (*gtk.Box, *modelUpload) {
	upload := &modelUpload{
		pathEntry: gtk.NewEntry(),
		button:    gtk.NewButtonWithLabel(t.UploadAndPrint),
	}
	upload.pathEntry.SetPlaceholderText(t.LocalGCodePath)
	upload.pathEntry.SetHExpand(true)
	row := gtk.NewBox(gtk.OrientationHorizontal, 6)
	row.Append(upload.pathEntry)
	chooseBtn := gtk.NewButtonWithLabel(t.SelectGCode)
	chooseBtn.ConnectClicked(func() { showGCodeFileChooser(parent, upload.pathEntry) })
	row.Append(chooseBtn)
	return row, upload
}

func (u *modelUpload) Path() string {
	return u.pathEntry.Text()
}

func (u *modelUpload) Reset() {
	u.pathEntry.SetText("")
}
