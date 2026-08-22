package widgets

import (
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
)

const (
	PreviewWidth  = 96
	PreviewHeight = 72
)

func LoadPreviewTexture(path string) *gdk.Texture {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	pixbuf, err := gdkpixbuf.NewPixbufFromFileAtScale(path, PreviewWidth, PreviewHeight, true)
	if err != nil {
		return nil
	}
	texture := gdk.NewTextureForPixbuf(pixbuf)
	return texture
}
