package shared

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/yanefed/filatrack/internal/i18n"
)

func ParseDecimal(text string) (float64, error) {
	value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(text), ",", "."), 64)
	if err == nil && (math.IsNaN(value) || math.IsInf(value, 0)) {
		err = fmt.Errorf("finite number required")
	}
	if err != nil {
		return 0, err
	}
	return value, nil
}

func Confirm(parent *gtk.Window, title, body, accept string, done func()) {
	t := i18n.L()
	dialog := adw.NewAlertDialog(title, body)
	cancelLabel := t.Cancel
	if accept == t.PrintAnyway {
		cancelLabel = t.ReturnToSelection
	}
	dialog.AddResponse("cancel", cancelLabel)
	dialog.AddResponse("accept", accept)
	dialog.SetDefaultResponse("cancel")
	dialog.SetCloseResponse("cancel")
	dialog.SetResponseAppearance("accept", adw.ResponseSuggested)
	if accept == t.Archive {
		dialog.SetResponseAppearance("accept", adw.ResponseDestructive)
	}
	dialog.ConnectResponse(func(response string) {
		if response == "accept" {
			done()
		}
	})
	if parent != nil {
		dialog.Present(parent)
	} else {
		dialog.Present(nil)
	}
}

// GuardChanges covers Cancel, Escape and the window close button. Reset after
// asynchronous initial values are applied; accepted writes block closing.
func GuardChanges(dialog *gtk.Window, snapshot func() string, busy func() bool) (close func(), reset func()) {
	initial := snapshot()
	pending := false
	close = func() {
		if busy != nil && busy() {
			return
		}
		if snapshot() == initial {
			dialog.Destroy()
			return
		}
		if pending {
			return
		}
		pending = true
		t := i18n.L()
		prompt := adw.NewAlertDialog(t.UnsavedTitle, t.UnsavedBody)
		prompt.AddResponse("cancel", t.KeepEditing)
		prompt.AddResponse("discard", t.Discard)
		prompt.SetDefaultResponse("cancel")
		prompt.SetCloseResponse("cancel")
		prompt.SetResponseAppearance("discard", adw.ResponseDestructive)
		prompt.ConnectResponse(func(response string) {
			pending = false
			if response == "discard" {
				dialog.Destroy()
			}
		})
		prompt.Present(dialog)
	}
	dialog.ConnectCloseRequest(func() bool { close(); return true })
	keys := gtk.NewEventControllerKey()
	keys.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if keyval == gdk.KEY_Escape {
			close()
			return true
		}
		return false
	})
	dialog.AddController(keys)
	return close, func() { initial = snapshot() }
}

func FormSnapshot(root gtk.Widgetter) string {
	var values []string
	var walk func(*gtk.Widget)
	walk = func(w *gtk.Widget) {
		switch value := w.Object.Cast().(type) {
		case *adw.EntryRow:
			values = append(values, value.Text())
			return
		case *adw.PasswordEntryRow:
			values = append(values, value.Text())
			return
		case *gtk.Entry:
			values = append(values, value.Text())
			return
		case *adw.ComboRow:
			values = append(values, fmt.Sprint(value.Selected()))
			return
		case *gtk.DropDown:
			values = append(values, fmt.Sprint(value.Selected()))
			return
		case *gtk.CheckButton:
			values = append(values, fmt.Sprint(value.Active()))
			return
		case *gtk.SpinButton:
			values = append(values, value.Text())
			return
		}
		for child := w.FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
			walk(gtk.BaseWidget(child))
		}
	}
	walk(gtk.BaseWidget(root))
	return strings.Join(values, "\x00")
}

func Notify(origin gtk.Widgetter, title, action string, run func()) {
	if origin == nil || reflect.ValueOf(origin).IsNil() {
		return
	}
	var overlay *adw.ToastOverlay
	var find func(gtk.Widgetter)
	find = func(current gtk.Widgetter) {
		if current == nil || overlay != nil {
			return
		}
		w := gtk.BaseWidget(current)
		if value, ok := w.Object.Cast().(*adw.ToastOverlay); ok {
			overlay = value
			return
		}
		for c := w.FirstChild(); c != nil; c = gtk.BaseWidget(c).NextSibling() {
			find(c)
		}
	}
	for current := origin; current != nil; current = gtk.BaseWidget(current).Parent() {
		w := gtk.BaseWidget(current)
		if value, ok := w.Object.Cast().(*adw.ToastOverlay); ok {
			overlay = value
		}
		if w.Parent() == nil {
			find(current)
		}
		if overlay != nil {
			toast := adw.NewToast(title)
			toast.SetUseMarkup(false)
			if run != nil {
				toast.SetPriority(adw.ToastPriorityHigh)
				toast.SetTimeout(10)
				toast.SetButtonLabel(action)
				toast.ConnectButtonClicked(run)
			}
			overlay.AddToast(toast)
			return
		}
	}
}

func SearchBox(root *gtk.Box, changed func()) *gtk.SearchEntry {
	t := i18n.L()
	search := gtk.NewSearchEntry()
	search.SetPlaceholderText(t.Search)
	search.SetSearchDelay(200)
	root.InsertChildAfter(search, root.FirstChild())
	AccessibleName(search, t.Search)
	search.ConnectSearchChanged(changed)
	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if (keyval == gdk.KEY_f || keyval == gdk.KEY_F) && state&(gdk.ControlMask|gdk.MetaMask) != 0 {
			search.GrabFocus()
			return true
		}
		return false
	})
	root.AddController(keys)
	return search
}

func InlineMessage(root *gtk.Box) *gtk.Label {
	label := gtk.NewLabel("")
	label.SetWrap(true)
	label.SetXAlign(0)
	label.SetVisible(false)
	root.Append(label)
	return label
}

func OpenPage(origin gtk.Widgetter, page string) {
	for current := origin; current != nil; current = gtk.BaseWidget(current).Parent() {
		if stack, ok := gtk.BaseWidget(current).Object.Cast().(*gtk.Stack); ok {
			stack.SetVisibleChildName(page)
			return
		}
	}
}

func FieldError(row *adw.EntryRow, title, message string, valid bool) {
	row.SetTitle(title)
	row.RemoveCSSClass("error")
	row.SetTooltipText("")
	if !valid {
		row.SetTitle(title + " · " + message)
		row.AddCSSClass("error")
		row.SetTooltipText(message)
	}
}

func FocusSearch(root gtk.Widgetter) bool {
	if root == nil {
		return false
	}
	w := gtk.BaseWidget(root)
	if search, ok := w.Object.Cast().(*gtk.SearchEntry); ok {
		return search.GrabFocus()
	}
	for child := w.FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		if FocusSearch(child) {
			return true
		}
	}
	return false
}

// EmptyAction stays aligned with the empty state rather than disappearing into
// a toolbar when the user has no records yet.
func EmptyAction(ctx context.Context, root *gtk.Box, empty *gtk.Label, title string, action func()) *gtk.Button {
	button := gtk.NewButtonWithLabel(title)
	button.SetHAlign(gtk.AlignCenter)
	button.SetVisible(empty.Visible())
	root.Append(button)
	empty.NotifyProperty("visible", func() { button.SetVisible(empty.Visible()) })
	button.ConnectClicked(func() {
		if ctx.Err() == nil {
			action()
		}
	})
	return button
}

func AccessibleName(target gtk.Widgetter, name string) {
	value := coreglib.NewValue(name)
	gtk.BaseWidget(target).Accessible.UpdateProperty([]gtk.AccessibleProperty{gtk.AccessiblePropertyLabel}, []coreglib.Value{*value})
}

func LabelIcons(root gtk.Widgetter) {
	w := gtk.BaseWidget(root)
	if b, ok := w.Object.Cast().(*gtk.Button); ok && b.Label() == "" && b.TooltipText() != "" {
		AccessibleName(b, b.TooltipText())
	}
	for c := w.FirstChild(); c != nil; c = gtk.BaseWidget(c).NextSibling() {
		LabelIcons(c)
	}
}
