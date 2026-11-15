package models

import (
	"strings"
	"testing"

	"github.com/yanefed/filatrack/internal/domain"
)

func TestModelKeysSafeForGTK(t *testing.T) {
	models := []domain.PrinterModelFile{{PrinterID: "p", Path: "first.gcode"}, {PrinterID: "p", Path: "second.gcode"}, {PrinterID: "p:second", Path: "gcode"}}
	seen := make(map[string]bool)
	for _, key := range modelKeys(models) {
		if strings.ContainsRune(key, 0) {
			t.Fatal("GtkStringList truncates keys at NUL, forcing list replacement on every refresh")
		}
		if seen[key] {
			t.Fatalf("duplicate model key: %q", key)
		}
		seen[key] = true
	}
}
