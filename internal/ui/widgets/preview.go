package widgets

import (
	"container/list"
	"fmt"
	"os"
	"sync"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	PreviewWidth  = 96
	PreviewHeight = 72
)

// Keep the full rendered print history so reversing scroll does not recreate textures.
const previewTextureCacheLimit = 256

type previewTextureEntry struct {
	key     string
	texture *gdk.Texture
}

var previewTextures = struct {
	sync.Mutex
	items map[string]*list.Element
	lru   list.List
}{items: make(map[string]*list.Element)}

// SetPreview loads a scaled preview into a picture, reusing a bounded texture cache.
func SetPreview(picture *gtk.Picture, path string) {
	if path == "" {
		picture.SetPaintable(nil)
		picture.SetVisible(false)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		picture.SetPaintable(nil)
		picture.SetVisible(false)
		return
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", path, info.Size(), info.ModTime().UnixNano())

	previewTextures.Lock()
	entry := previewTextures.items[key]
	if entry != nil {
		previewTextures.lru.MoveToFront(entry)
		texture := entry.Value.(previewTextureEntry).texture
		previewTextures.Unlock()
		picture.SetPaintable(texture)
		picture.SetVisible(true)
		return
	}
	previewTextures.Unlock()

	pixbuf, err := gdkpixbuf.NewPixbufFromFileAtScale(path, PreviewWidth, PreviewHeight, true)
	if err != nil {
		picture.SetPaintable(nil)
		picture.SetVisible(false)
		return
	}
	texture := gdk.NewTextureForPixbuf(pixbuf)
	coreglib.Destroy(pixbuf)
	if texture == nil {
		picture.SetPaintable(nil)
		picture.SetVisible(false)
		return
	}

	previewTextures.Lock()
	// Another bind may have loaded the same file while decoding happened.
	if entry = previewTextures.items[key]; entry != nil {
		previewTextures.lru.MoveToFront(entry)
		cached := entry.Value.(previewTextureEntry).texture
		previewTextures.Unlock()
		coreglib.Destroy(texture)
		picture.SetPaintable(cached)
		picture.SetVisible(true)
		return
	}
	entry = previewTextures.lru.PushFront(previewTextureEntry{key: key, texture: texture})
	previewTextures.items[key] = entry
	for previewTextures.lru.Len() > previewTextureCacheLimit {
		oldest := previewTextures.lru.Back()
		old := oldest.Value.(previewTextureEntry)
		delete(previewTextures.items, old.key)
		previewTextures.lru.Remove(oldest)
		coreglib.Destroy(old.texture)
	}
	previewTextures.Unlock()

	picture.SetPaintable(texture)
	picture.SetVisible(true)
}

// PreviewTextureCacheSize reports the number of scaled textures retained by the cache.
func PreviewTextureCacheSize() int {
	previewTextures.Lock()
	defer previewTextures.Unlock()
	return previewTextures.lru.Len()
}
