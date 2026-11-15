package shared

import (
	"reflect"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// SyncStrings retains the unchanged prefix/suffix, including the current scroll anchor.
func SyncStrings(model *gtk.StringList, keys []string) {
	old := int(model.NItems())
	start := 0
	for start < old && start < len(keys) && model.String(uint(start)) == keys[start] {
		start++
	}
	endOld, endNew := old, len(keys)
	for endOld > start && endNew > start && model.String(uint(endOld-1)) == keys[endNew-1] {
		endOld--
		endNew--
	}
	if start != endOld || start != endNew {
		model.Splice(uint(start), uint(endOld-start), keys[start:endNew])
	}
}

type virtualView interface {
	SetModel(gtk.SelectionModeller)
	SetFactory(*gtk.ListItemFactory)
}

// Virtual retains each visible card's widgets and updates bindings independently
// of structural list changes. Action closures must read the most recently bound value.
func Virtual[T any](view virtualView, key func(T) string, create func() (gtk.Widgetter, func(T))) func([]T) {
	model := gtk.NewStringList(nil)
	factory := gtk.NewSignalListItemFactory()
	type row struct {
		item     *gtk.ListItem
		render   func(T)
		previous T
		bound    bool
	}
	rows := make(map[uintptr]*row)
	var items []T
	bind := func(r *row) {
		i := r.item.Position()
		if int(i) >= len(items) {
			return
		}
		value := items[i]
		if r.bound && reflect.DeepEqual(r.previous, value) {
			return
		}
		r.render(value)
		r.previous = value
		r.bound = true
	}
	factory.ConnectSetup(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		widget, render := create()
		item.SetChild(widget)
		item.SetActivatable(false)
		item.SetFocusable(false)
		rows[item.Native()] = &row{item: item, render: render}
	})
	factory.ConnectBind(func(object *coreglib.Object) {
		if r := rows[object.Native()]; r != nil {
			bind(r)
		}
	})
	factory.ConnectUnbind(func(object *coreglib.Object) {
		if r := rows[object.Native()]; r != nil {
			r.bound = false
			var zero T
			r.previous = zero
		}
	})
	factory.ConnectTeardown(func(object *coreglib.Object) { delete(rows, object.Native()) })
	view.SetModel(gtk.NewNoSelection(model))
	view.SetFactory(&factory.ListItemFactory)
	return func(values []T) {
		items = values
		keys := make([]string, len(items))
		for i, value := range items {
			keys[i] = key(value)
		}
		SyncStrings(model, keys)
		for _, r := range rows {
			bind(r)
		}
	}
}
