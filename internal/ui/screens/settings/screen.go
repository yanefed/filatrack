package settings

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/widgets"
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// NewSettingsScreen создаёт экран настроек приложения.
func NewSettingsScreen(ctx context.Context, parent *gtk.Window, settingsSvc *service.SettingsService, onChanged func(), onLanguageChanged func()) (*gtk.Box, func()) {
	box := widgets.NewScreen(i18n.L().Settings)

	content := gtk.NewBox(gtk.OrientationVertical, 18)
	content.SetMarginTop(12)
	box.Append(widgets.NewScrolled(content))

	content.Append(newLanguageSettingsSection(ctx, settingsSvc, onLanguageChanged))
	content.Append(newCurrencySettingsSection(ctx, settingsSvc, onChanged))
	content.Append(newPrintSettingsSection(ctx, settingsSvc, onChanged))

	matSection, refreshMaterials := newMaterialsSection(ctx, parent, settingsSvc, onChanged)
	content.Append(matSection)

	refreshSettings := func() {
		refreshMaterials()
	}

	return box, refreshSettings
}
