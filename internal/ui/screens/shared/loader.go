package shared

import (
	"context"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/ui/task"
)

func Dispatch(apply func()) { coreglib.IdleAdd(apply) }

// NewLoader keeps database work out of GTK and defers hidden screens until mapped.
func NewLoader[T any](ctx context.Context, root gtk.Widgetter) *task.Latest[T] {
	life, cancel := context.WithCancel(ctx)
	widget := gtk.BaseWidget(root)
	loader := task.NewLatest[T](life, Dispatch)
	widget.ConnectMap(loader.Resume)
	widget.ConnectUnmap(loader.Pause)
	widget.ConnectDestroy(cancel)
	if widget.Mapped() {
		loader.Resume()
	}
	return loader
}

// Action prevents duplicate writes and suppresses callbacks after destruction.
// Hiding a widget does not cancel an already accepted write.
func Action(ctx context.Context, root gtk.Widgetter) func(func() error, func(error)) {
	closed, busy := false, false
	gtk.BaseWidget(root).ConnectDestroy(func() { closed = true })
	return func(work func() error, done func(error)) {
		if closed || busy || ctx.Err() != nil {
			return
		}
		busy = true
		go func() {
			err := work()
			Dispatch(func() {
				busy = false
				if !closed && ctx.Err() == nil {
					done(err)
				}
			})
		}()
	}
}
