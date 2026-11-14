package widgets

import (
	"container/list"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	PreviewWidth             = 96
	PreviewHeight            = 72
	previewTextureCacheLimit = 256
)

type previewTextureEntry struct {
	key     string
	texture *gdk.Texture
}

var previewTextures = struct {
	sync.Mutex
	items map[string]*list.Element
	lru   list.List
}{items: make(map[string]*list.Element)}

type previewTarget struct {
	version atomic.Uint64
	closed  atomic.Bool
}
type previewRequest struct {
	picture *gtk.Picture
	target  *previewTarget
	version uint64
}

func (r previewRequest) current() bool {
	return !r.target.closed.Load() && r.target.version.Load() == r.version
}

// Targets and pending decodes are owned by GTK. Workers only inspect atomic tokens.
var previewTargets = make(map[uintptr]*previewTarget)
var pendingPreviews = make(map[string][]previewRequest)
var previewJobs = make(chan func(), 256)
var previewWorkers sync.Once

func previewDispatch(work func()) { coreglib.IdleAdd(work) }

func enqueuePreview(work func(), current func() bool) {
	if current != nil && !current() {
		return
	}
	previewWorkers.Do(func() {
		for range 2 {
			go func() {
				for job := range previewJobs {
					job()
				}
			}()
		}
	})
	select {
	case previewJobs <- work:
	default:
		// Never wait for file I/O on the GTK thread, even during a burst of binds.
		coreglib.IdleAdd(func() bool {
			if current != nil && !current() {
				return false
			}
			select {
			case previewJobs <- work:
				return false
			default:
				return true
			}
		})
	}
}

// SetPreview schedules stat/decode work; creating and applying textures stays in GTK.
// Rebinding or clearing a picture invalidates every earlier completion for it.
func SetPreview(picture *gtk.Picture, path string) {
	native := picture.Object.Native()
	target := previewTargets[native]
	if target == nil {
		target = &previewTarget{}
		previewTargets[native] = target
		picture.ConnectDestroy(func() { target.closed.Store(true); target.version.Add(1); delete(previewTargets, native) })
	}
	request := previewRequest{picture: picture, target: target, version: target.version.Add(1)}
	if path == "" {
		picture.SetPaintable(nil)
		picture.SetVisible(false)
		return
	}
	enqueuePreview(func() {
		if !request.current() {
			return
		}
		info, err := os.Stat(path)
		previewDispatch(func() {
			if !request.current() {
				return
			}
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
			if pending, exists := pendingPreviews[key]; exists {
				pendingPreviews[key] = append(pending, request)
				return
			}
			pendingPreviews[key] = []previewRequest{request}
			enqueuePreview(func() {
				pixbuf, decodeErr := gdkpixbuf.NewPixbufFromFileAtScale(path, PreviewWidth, PreviewHeight, true)
				previewDispatch(func() {
					requests := pendingPreviews[key]
					delete(pendingPreviews, key)
					active := false
					for _, req := range requests {
						active = active || req.current()
					}
					if decodeErr != nil {
						for _, req := range requests {
							if req.current() {
								req.picture.SetPaintable(nil)
								req.picture.SetVisible(false)
							}
						}
						return
					}
					defer coreglib.Destroy(pixbuf)
					if !active {
						return
					}
					texture := gdk.NewTextureForPixbuf(pixbuf)
					if texture == nil {
						return
					}
					previewTextures.Lock()
					entry := previewTextures.lru.PushFront(previewTextureEntry{key, texture})
					previewTextures.items[key] = entry
					for previewTextures.lru.Len() > previewTextureCacheLimit {
						oldest := previewTextures.lru.Back()
						old := oldest.Value.(previewTextureEntry)
						delete(previewTextures.items, old.key)
						previewTextures.lru.Remove(oldest)
						coreglib.Destroy(old.texture)
					}
					previewTextures.Unlock()
					for _, req := range requests {
						if req.current() {
							req.picture.SetPaintable(texture)
							req.picture.SetVisible(true)
						}
					}
				})
			}, nil)
		})
	}, request.current)
}
func PreviewTextureCacheSize() int {
	previewTextures.Lock()
	defer previewTextures.Unlock()
	return previewTextures.lru.Len()
}
