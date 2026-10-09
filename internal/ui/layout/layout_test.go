package layout

import (
	"encoding/xml"
	"io"
	"io/fs"
	"runtime"
	"strings"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func initDisplay(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
}

// Exercise every view, including dialogs and rows that screen constructors don't open.
func TestAllViewsAndComponentsLoad(t *testing.T) {
	initDisplay(t)
	for _, filename := range viewFiles(t) {
		t.Run(filename, func(t *testing.T) {
			Load(strings.TrimSuffix(strings.TrimPrefix(filename, "views/"), ".ui"))
		})
	}

	for name, component := range components {
		t.Run(name+" component", func(t *testing.T) {
			view := Load(name)
			for _, root := range component.Roots {
				if view.builder.GetObject(root) == nil {
					t.Fatalf("missing component root %s in %s", root, component.File)
				}
			}
		})
	}
}

// IDs must stay unique and component contracts must refer to the actual source file.
func TestComponentContracts(t *testing.T) {
	ids := map[string]map[string]bool{}
	for _, filename := range viewFiles(t) {
		file, err := views.Open(filename)
		if err != nil {
			t.Fatal(err)
		}

		decoder := xml.NewDecoder(file)
		found := map[string]bool{}
		for {
			tok, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "object" {
				for _, attr := range start.Attr {
					if attr.Name.Local == "id" {
						if found[attr.Value] {
							t.Fatalf("duplicate ID %s in %s", attr.Value, filename)
						}
						found[attr.Value] = true
					}
				}
			}
		}
		file.Close()
		ids[strings.TrimSuffix(strings.TrimPrefix(filename, "views/"), ".ui")] = found
	}
	for name, component := range components {
		for _, root := range component.Roots {
			if !ids[component.File][root] {
				t.Errorf("%s: missing root %s in %s", name, root, component.File)
			}
		}
	}
}

func TestMainLayoutNavigationAndToolbar(t *testing.T) {
	initDisplay(t)
	view := Load("main-layout")
	stack := Get[*gtk.Stack](view, "stack")
	for _, name := range []string{"dashboard", "spools", "prints", "models", "drying", "printers", "settings"} {
		page := Get[*gtk.Box](Scope(view, name), "box")
		if stack.ChildByName(name) == nil {
			t.Fatalf("missing page %s", name)
		}
		stack.SetVisibleChildName(name)
		if stack.Page(page).Name() != name {
			t.Fatalf("page %s is not in the navigation stack", name)
		}
	}
	for _, control := range []string{"addSpoolButton", "prints_syncHistoryBtn", "prints_printerCombo", "models_addModelBtn", "models_refreshModelsBtn", "drying_addDryingButton", "printers_addPrinterButton"} {
		widget := Get[gtk.Widgetter](view, control)
		if gtk.BaseWidget(widget).Parent() == nil {
			t.Fatalf("toolbar control %s has no parent", control)
		}
	}
}

// Visit all nested source directories, not just files at the views root.
func viewFiles(t *testing.T) []string {
	t.Helper()
	var filenames []string
	err := fs.WalkDir(views, "views", func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(filename, ".ui") {
			filenames = append(filenames, filename)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return filenames
}
