package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
		var zero T
		return zero
	}
}
func TestLatestDefersUntilVisibleAndReplacesQueuedJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dispatched := make(chan func(), 10)
	started := make(chan int, 10)
	applied := make(chan int, 10)
	loader := NewLatest[int](ctx, func(f func()) { dispatched <- f })
	work := func(n int) func(context.Context) int { return func(context.Context) int { started <- n; return n } }
	loader.Request(work(1), func(n int) { applied <- n })
	loader.Request(work(2), func(n int) { applied <- n })
	select {
	case <-started:
		t.Fatal("hidden view loaded data")
	default:
	}
	loader.Resume()
	if n := receive(t, started); n != 2 {
		t.Fatal(n)
	}
	receive(t, dispatched)()
	if n := receive(t, applied); n != 2 {
		t.Fatal(n)
	}
	loader.Pause()
	loader.Resume()
	select {
	case <-started:
		t.Fatal("clean view reloaded on map")
	default:
	}
}
func TestLatestCancelsRunningReadAndSuppressesStaleCompletion(t *testing.T) {
	dispatched := make(chan func(), 10)
	started := make(chan int, 10)
	release := make(chan struct{})
	cancelled := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	loader := NewLatest[int](ctx, func(f func()) { dispatched <- f })
	loader.Resume()
	var active, max atomic.Int32
	loader.Request(func(ctx context.Context) int {
		n := active.Add(1)
		max.Store(n)
		started <- 1
		<-ctx.Done()
		close(cancelled)
		<-release
		active.Add(-1)
		return 1
	}, func(int) { t.Error("stale result applied") })
	receive(t, started)
	loader.Request(func(context.Context) int {
		n := active.Add(1)
		if n > max.Load() {
			max.Store(n)
		}
		defer active.Add(-1)
		started <- 2
		return 2
	}, func(int) {})
	receive(t, cancelled)
	select {
	case <-started:
		t.Fatal("overlapping reads")
	default:
	}
	close(release)
	if n := receive(t, started); n != 2 {
		t.Fatal(n)
	}
	receive(t, dispatched)()
	if max.Load() != 1 {
		t.Fatal("concurrency limit failed")
	}
}
func TestLatestPauseInvalidatesDispatchedCallbackAndRetriesOnResume(t *testing.T) {
	dispatched := make(chan func(), 10)
	applied := 0
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	loader := NewLatest[int](ctx, func(f func()) { dispatched <- f })
	loader.Resume()
	loader.Request(func(context.Context) int { return 1 }, func(int) { applied++ })
	stale := receive(t, dispatched)
	loader.Pause()
	stale()
	if applied != 0 {
		t.Fatal("hidden view updated")
	}
	loader.Resume()
	receive(t, dispatched)()
	if applied != 1 {
		t.Fatal(applied)
	}
	loader.Request(func(context.Context) int { return 2 }, func(int) { t.Error("closed view updated") })
	callback := receive(t, dispatched)
	cancel()
	callback()
}

func TestPollDoesNotStarveSlowReadsOrSupersedeReadyResults(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dispatched := make(chan func(), 10)
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	applied := false
	loader := NewLatest[int](ctx, func(f func()) { dispatched <- f })
	loader.Resume()
	work := func(ctx context.Context) int { started <- ctx; <-release; return 1 }
	loader.Poll(work, func(int) { applied = true })
	running := receive(t, started)
	loader.Poll(func(context.Context) int { t.Error("duplicate poll"); return 2 }, func(int) {})
	if running.Err() != nil {
		t.Fatal("timer cancelled an in-flight read")
	}
	close(release)
	callback := receive(t, dispatched)
	loader.Poll(func(context.Context) int { t.Error("ready result superseded"); return 3 }, func(int) {})
	callback()
	if !applied {
		t.Fatal("ready result not applied")
	}
}
