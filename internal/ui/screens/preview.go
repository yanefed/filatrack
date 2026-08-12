package screens

import (
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
)

const (
	printsRenderLimit = 250
	previewWidth      = 96
	previewHeight     = 72
)

func loadScaledPreviewTexture(path string) *gdk.Texture {
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
	return texture
}
