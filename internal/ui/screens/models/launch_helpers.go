package models

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"context"
	"fmt"

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
	case result.Uploaded:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchUploaded, result.Filename))
	case result.Started || result.Queued:
		label.SetLabel(fmt.Sprintf("%s: %s", t.LaunchStarted, result.Filename))
	default:
		label.SetLabel(fmt.Sprintf("%s: %.1fg / %.1fg", t.LaunchWarning, result.Filament.RequiredWeightG, result.Filament.RemainingBeforeG))
	}
	label.SetVisible(true)
}

func showGCodeFileChooser(parent *gtk.Window, pathEntry *gtk.Entry) {
	t := i18n.L()
	dialog := gtk.NewFileDialog()
	dialog.SetTitle(t.SelectGCode)
	dialog.SetAcceptLabel(t.SelectGCode)
	filter := gtk.NewFileFilter()
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
		pathEntry.SetText(path)
	})
}
