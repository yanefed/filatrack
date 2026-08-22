package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestModelsListIsVirtualized(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	const count = 1000
	entries := make([]domain.PrinterModelFile, count)
	for i := range entries {
		entries[i] = domain.PrinterModelFile{PrinterID: "printer", Path: fmt.Sprintf("model-%d.gcode", i)}
	}
	view, update := newVirtualModelsList(i18n.L(), context.Background(), nil, nil, nil, gtk.NewLabel(""), nil)
	update(entries, nil, nil)
	window := gtk.NewWindow()
	window.SetDefaultSize(800, 600)
	window.SetChild(wrapList(view))
	window.Present()
	iterateGTK()
	if widgets := descendantCount(view); widgets >= count*3 {
		t.Fatalf("models view created %d widgets for %d records", widgets, count)
	}
	window.Destroy()
}

func wrapList(view *gtk.ListView) *gtk.ScrolledWindow {
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetChild(view)
	return scrolled
}

func iterateGTK() {
	mainContext := glib.MainContextDefault()
	for i := 0; i < 20; i++ {
		mainContext.Iteration(false)
	}
}

func descendantCount(root gtk.Widgetter) int {
	count := 0
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		count++
		count += descendantCount(child)
	}
	return count
}
