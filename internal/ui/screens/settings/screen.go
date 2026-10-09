package settings

import (
	"github.com/yanefed/filatrack/internal/database"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"

	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type DatabaseOptions struct {
	Manager    *database.Manager
	OnSwitched func()
	View       *layout.View
}

// NewSettingsScreen создаёт экран настроек приложения.
func NewSettingsScreen(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func(), onLanguageChanged func(), databaseOptions ...DatabaseOptions) (*gtk.Box, func()) {
	var existingView *layout.View
	if len(databaseOptions) > 0 {
		existingView = databaseOptions[0].View
	}
	view := layout.Use("settings-screen", existingView)

	box := layout.Configure[*gtk.Box](view, "box", func(w *gtk.Box) { layout.Get[*gtk.Label](view, "box_title").SetLabel(i18n.L().Settings) })

	databaseSection := layout.Get[*gtk.Box](view, "database_section_section")
	databaseSection.SetVisible(len(databaseOptions) > 0 && databaseOptions[0].Manager != nil)

	_ = newLanguageSettingsSection(ctx, settingsSvc, onLanguageChanged, layout.Scope(view, "content_slot_1"))
	_ = newCurrencySettingsSection(ctx, settingsSvc, onChanged, layout.Scope(view, "content_slot_2"))
	_ = newPrintSettingsSection(ctx, settingsSvc, onChanged, layout.Scope(view, "content_slot_3"))
	if len(databaseOptions) > 0 && databaseOptions[0].Manager != nil {
		_ = newDatabaseSection(ctx, parent, databaseOptions[0], layout.Scope(view, "database_section"))
	}

	_, refreshMaterials := newMaterialsSection(ctx, parent, settingsSvc, onChanged, layout.Scope(view, "content_slot_4"))

	refreshSettings := func() {
		refreshMaterials()
	}

	return box, refreshSettings
}
