// Package task coordinates background reads without depending on GTK.
package task

import (
	"context"
	"sync"
)

type job[T any] struct {
	work  func(context.Context) T
	apply func(T)
}

// Latest runs one job at a time, replacing any queued job with the newest request.
// dispatch must schedule a function on the UI thread, never execute it inline.
type Latest[T any] struct {
	mu                  sync.Mutex
	ctx                 context.Context
	dispatch            func(func())
	active, running     bool
	closed              bool
	generation, applied uint64
	cancel              context.CancelFunc
	pending, last       *job[T]
}

func NewLatest[T any](ctx context.Context, dispatch func(func())) *Latest[T] {
	l := &Latest[T]{ctx: ctx, dispatch: dispatch}
	context.AfterFunc(ctx, l.Close)
	return l
}
func (l *Latest[T]) Request(work func(context.Context) T, apply func(T)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.request(work, apply)
}

func (l *Latest[T]) request(work func(context.Context) T, apply func(T)) {
	if l.closed || l.ctx.Err() != nil {
		return
	}
	l.generation++
	l.last = &job[T]{work, apply}
	l.pending = l.last
	if l.cancel != nil {
		l.cancel()
	}
	l.start()
}

// Poll coalesces timer ticks without repeatedly cancelling a slow read or
// superseding a result already queued for the UI thread.
func (l *Latest[T]) Poll(work func(context.Context) T, apply func(T)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running || l.pending != nil || l.applied != l.generation {
		return
	}
	l.request(work, apply)
}

func (l *Latest[T]) Resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.active = true
	l.start()
}
func (l *Latest[T]) Pause() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.active = false
	if l.applied != l.generation {
		l.pending = l.last
	}
	l.generation++
	// A completed result remains current across ordinary tab switches.
	if l.pending == nil {
		l.applied = l.generation
	}
	if l.cancel != nil {
		l.cancel()
	}
}
func (l *Latest[T]) start() {
	if l.closed || !l.active || l.running || l.pending == nil || l.ctx.Err() != nil {
		return
	}
	j, version := l.pending, l.generation
	l.pending = nil
	l.running = true
	ctx, cancel := context.WithCancel(l.ctx)
	l.cancel = cancel
	go func() {
		value := j.work(ctx)
		l.mu.Lock()
		l.running = false
		l.cancel = nil
		valid := ctx.Err() == nil && l.active && version == l.generation
		l.start()
		l.mu.Unlock()
		cancel()
		if !valid {
			return
		}
		l.dispatch(func() {
			l.mu.Lock()
			valid := l.ctx.Err() == nil && l.active && version == l.generation
			if valid {
				l.applied = version
			}
			l.mu.Unlock()
			if valid {
				j.apply(value)
			}
		})
	}()
}

// Close releases queued work and captured widgets when a view is rebuilt.
func (l *Latest[T]) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	l.active = false
	l.generation++
	l.pending = nil
	l.last = nil
	if l.cancel != nil {
		l.cancel()
	}
}
