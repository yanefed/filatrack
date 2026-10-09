// Package layout loads the GtkBuilder views edited in the Cambalache project.
package layout

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed views styles/*.css components.json
var views embed.FS

// View binds a controller to an independent UI instance or one of its subtrees.
type View struct {
	builder *gtk.Builder
	prefix  string
}

type component struct {
	File   string   `json:"file"`
	Prefix string   `json:"prefix"`
	Roots  []string `json:"roots"`
}

// Scope binds a component controller to a subtree of an already loaded view.
func Scope(view *View, prefix string) *View {
	return &View{builder: view.builder, prefix: view.prefix + prefix + "_"}
}

// Use uses an existing component subtree, or creates a standalone instance.
func Use(name string, views ...*View) *View {
	if len(views) > 0 && views[0] != nil {
		return views[0]
	}
	return Load(name)
}

// Load creates an independent instance of a view, dialog, or repeated card.
func Load(name string) *View {
	adw.Init()
	entry, found := components[name]
	if !found {
		entry.File = name
	}
	data, err := views.ReadFile("views/" + entry.File + ".ui")
	if err != nil {
		panic(fmt.Errorf("reading UI %s: %w", name, err))
	}
	builder := gtk.NewBuilder()
	var loadErr error
	if len(entry.Roots) > 0 {
		loadErr = builder.AddObjectsFromString(string(data), entry.Roots)
	} else {
		loadErr = builder.AddFromString(string(data))
	}
	if err := loadErr; err != nil {
		panic(fmt.Errorf("loading UI %s: %w", name, err))
	}
	return &View{builder: builder, prefix: entry.Prefix}
}

// Get checks the ID/type contract between the visual view and its controller.
func Get[T any](view *View, id string) T {
	object := view.builder.GetObject(view.prefix + id)
	if object == nil {
		panic(fmt.Sprintf("UI: missing object %q", id))
	}
	value, ok := object.Cast().(T)
	if !ok {
		panic(fmt.Sprintf("UI: unexpected type for %q", id))
	}
	return value
}

// Configure attaches runtime data to an object whose layout lives in a .ui file.
func Configure[T any](view *View, id string, configure func(T)) T {
	value := Get[T](view, id)
	configure(value)
	return value
}

// Mount fills a declarative container with runtime content or a separate component.
func Mount(view *View, id string, child gtk.Widgetter) {
	Get[*gtk.Box](view, id).Append(child)
}

// CSS returns an embedded stylesheet edited alongside the Cambalache views.
func CSS(name string) string {
	data, err := views.ReadFile("styles/" + name + ".css")
	if err != nil {
		panic(fmt.Errorf("reading UI stylesheet %s: %w", name, err))
	}
	return string(data)
}

var components = func() map[string]component {
	data, err := views.ReadFile("components.json")
	if err != nil {
		panic(err)
	}
	var catalog map[string]component
	if err := json.Unmarshal(data, &catalog); err != nil {
		panic(err)
	}
	return catalog
}()
