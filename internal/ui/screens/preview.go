package screens

import (
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	printsRenderLimit = 250
	previewWidth      = 96
	previewHeight     = 72
)

func newScaledPreview(path string) *gtk.Picture {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	pixbuf, err := gdkpixbuf.NewPixbufFromFileAtScale(path, previewWidth, previewHeight, true)
	if err != nil {
		return nil
	}
	texture := gdk.NewTextureForPixbuf(pixbuf)
	preview := gtk.NewPictureForPaintable(texture)
	preview.SetSizeRequest(previewWidth, previewHeight)
	preview.SetContentFit(gtk.ContentFitContain)
	return preview
}
