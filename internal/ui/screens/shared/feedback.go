package shared

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/i18n"
)

// ReadFeedback keeps loading and read errors separate from empty results.
// It never clears data widgets while a request is in flight or has failed.
type ReadFeedback struct {
	label  *gtk.Label
	retry  *gtk.Button
	loaded bool
}

func NewReadFeedback(root *gtk.Box, label *gtk.Label, retry func()) *ReadFeedback {
	button := gtk.NewButtonWithLabel(i18n.L().Retry)
	button.SetVisible(false)
	button.SetHAlign(gtk.AlignCenter)
	root.Append(button)
	root.ReorderChildAfter(button, label)
	button.ConnectClicked(retry)
	return &ReadFeedback{label: label, retry: button}
}
func (f *ReadFeedback) Begin() {
	text := i18n.L().Loading
	if f.loaded {
		text = i18n.L().Refreshing
	}
	f.label.SetText(text)
	f.label.SetVisible(true)
	f.retry.SetVisible(false)
}
func (f *ReadFeedback) Success() {
	f.loaded = true
	f.label.SetVisible(false)
	f.retry.SetVisible(false)
}
func (f *ReadFeedback) Failure(err error) {
	f.FailureMessage(fmt.Sprintf("%s: %v", i18n.L().ErrorPrefix, err))
}
func (f *ReadFeedback) FailureMessage(message string) {
	f.label.SetText(message)
	f.label.SetVisible(true)
	f.retry.SetVisible(true)
}
