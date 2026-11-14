package shared

import "context"

// RunAsyncUI checks the view lifetime on both sides of a background operation.
func RunAsyncUI(ctx context.Context, work func(), done func()) {
	go func() {
		if ctx.Err() != nil {
			return
		}
		if work != nil {
			work()
		}
		if done == nil {
			return
		}
		Dispatch(func() {
			if ctx.Err() == nil {
				done()
			}
		})
	}()
}
