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
		text := fmt.Sprintf("%s: %v", t.ErrorPrefix, err)
		if result != nil {
			if result.SpoolAssigned {
				text = t.LaunchAssignUnconfirmed + "\n" + text
			} else if result.LaunchAttempted {
				text = t.LaunchUnconfirmed + "\n" + text
			}
		}
		label.SetText(text)
		label.SetVisible(true)
		return
	}
	if result == nil {
		return
	}
	if result.Started || result.Queued || result.Uploaded {
		label.SetText(launchSuccessText(t, result))
	} else {
		label.SetText(filamentText(t, result.Filament))
	}
	label.SetVisible(true)
}

type launchPreview struct {
	result *domain.PrintLaunchResult
	err    error
}

func filamentText(t i18n.T, check domain.FilamentCheck) string {
	if check.Blocked {
		return fmt.Sprintf(t.FilamentInsufficientFmt, check.RequiredWeightG, check.RemainingBeforeG)
	}
	if !check.UsageKnown {
		return t.FilamentUnknown
	}
	text := fmt.Sprintf(t.FilamentEstimateFmt, check.RequiredWeightG, check.RemainingBeforeG, check.RemainingAfterG)
	if check.Warning != "" {
		text += "\n" + fmt.Sprintf(t.FilamentLowFmt, check.RemainingAfterG)
	}
	return text
}
func launchSuccessText(t i18n.T, result *domain.PrintLaunchResult) string {
	state := t.LaunchUploaded
	if result.Queued {
		state = t.LaunchQueued
	} else if result.Started {
		state = t.LaunchStarted
	}
	return state + ": " + result.Filename
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
