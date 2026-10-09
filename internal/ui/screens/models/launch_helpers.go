package models

import (
	"context"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func selectedDirectSpool(dropdown *gtk.DropDown, spools []domain.Spool) string {
	idx := dropdown.Selected()
	if int(idx) >= len(spools) {
		return ""
	}
	return spools[idx].ID
}

func showLaunchResult(t i18n.T, label *gtk.Label, result *domain.PrintLaunchResult, err error) {
	if err != nil {
		label.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
		label.SetVisible(true)
		return
	}
	if result == nil {
		return
	}
	switch {
	case result.Filament.Blocked:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchBlocked, result.Filament.Warning))
	case result.RequiresUserConfirmation:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchWarning, result.Filament.Warning))
	case result.Started || result.Queued:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchStarted, result.Filename))
	case result.Uploaded:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchUploaded, result.Filename))
	default:
		label.SetLabel(fmt.Sprintf("%s: %.1fg / %.1fg", t.LaunchWarning, result.Filament.RequiredWeightG, result.Filament.RemainingBeforeG))
	}
	label.SetVisible(true)
}

func showGCodeFileChooser(parent *gtk.Window, onSelected func(string)) {
	view := layout.Load("gcode-file-dialog")

	t := i18n.L()
	dialog := layout.Get[*gtk.FileDialog](view, "dialog")
	dialog.SetTitle(t.SelectGCode)
	dialog.SetAcceptLabel(t.SelectGCode)
	filter := layout.Get[*gtk.FileFilter](view, "filter")
	for _, suffix := range []string{"gcode", "gco", "gc"} {
		filter.AddSuffix(suffix)
	}
	for _, pattern := range []string{"*.gcode", "*.GCODE", "*.gco", "*.GCO", "*.gc", "*.GC"} {
		filter.AddPattern(pattern)
	}
	dialog.SetDefaultFilter(filter)
	dialog.Open(context.Background(), parent, func(result gio.AsyncResulter) {
		file, err := dialog.OpenFinish(result)
		if err != nil || file == nil {
			return
		}
		path := file.Path()
		if path == "" {
			path = file.ParseName()
		}
		if onSelected != nil {
			onSelected(path)
		}
	})
}
