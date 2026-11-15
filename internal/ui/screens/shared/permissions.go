package shared

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/i18n"
)

type readOnlyKey struct{}

func WithReadOnly(ctx context.Context, readOnly bool) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, readOnly)
}
func ReadOnly(ctx context.Context) bool { value, _ := ctx.Value(readOnlyKey{}).(bool); return value }

// Keep offline actions disabled when an asynchronous refresh changes sensitivity.
func ProtectWrites(ctx context.Context, target gtk.Widgetter) {
	if !ReadOnly(ctx) {
		return
	}
	widget := gtk.BaseWidget(target)
	widget.SetSensitive(false)
	widget.SetTooltipText(i18n.L().OfflineAction)
	widget.NotifyProperty("sensitive", func() {
		if widget.Sensitive() {
			widget.SetSensitive(false)
		}
	})
}
