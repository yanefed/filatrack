package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Check production imports so test fixtures can still use real adapters.
func TestLayerDependencies(t *testing.T) {
	const internalPrefix = "github.com/yanefed/filatrack/internal/"
	for _, layer := range []string{"domain", "port", "service", "ui"} {
		t.Run(layer, func(t *testing.T) {
			root := filepath.Join("..", layer)
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					return err
				}
				for _, spec := range file.Imports {
					dependency, err := strconv.Unquote(spec.Path.Value)
					if err != nil {
						return err
					}
					internal := strings.TrimPrefix(dependency, internalPrefix)
					forbidden := false
					switch layer {
					case "domain", "port":
						// The inner contracts depend only on the domain and standard library.
						forbidden = strings.Contains(strings.Split(dependency, "/")[0], ".") &&
							!(layer == "port" && dependency == internalPrefix+"domain")
					case "service":
						for _, adapter := range []string{"repository", "database", "moonraker", "ui"} {
							forbidden = forbidden || internal == adapter || strings.HasPrefix(internal, adapter+"/")
						}
					case "ui":
						forbidden = internal == "port" || internal == "repository" || strings.HasPrefix(internal, "repository/")
					}
					if forbidden {
						t.Errorf("%s: %s must not depend on %s", path, layer, dependency)
					}
				}
				if layer == "port" {
					for _, declaration := range file.Decls {
						group, ok := declaration.(*ast.GenDecl)
						if !ok || group.Tok != token.TYPE {
							continue
						}
						for _, spec := range group.Specs {
							typeSpec := spec.(*ast.TypeSpec)
							if _, ok := typeSpec.Type.(*ast.InterfaceType); !ok {
								t.Errorf("%s: port %s must be an interface; data belongs to its owning layer", path, typeSpec.Name)
							}
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
