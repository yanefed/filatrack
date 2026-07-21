package screens

import "github.com/diamondburned/gotk4/pkg/glib/v2"

func runAsyncUI(work func(), done func()) {
	go func() {
		if work != nil {
			work()
		}
		if done == nil {
			return
		}
		glib.MainContextDefault().InvokeFull(glib.PRIORITY_DEFAULT_IDLE, func() bool {
			done()
			return false
		})
	}()
}
