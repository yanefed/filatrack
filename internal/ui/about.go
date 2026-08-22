package ui

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/packaging/metadata"
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type openSourceComponent struct {
	name      string
	version   string
	license   string
	copyright string
	sourceURL string
}

var openSourceComponents = []openSourceComponent{
	{name: "GTK", version: "4", license: "LGPL-2.1-or-later", sourceURL: "https://gitlab.gnome.org/GNOME/gtk"},
	{name: "libadwaita", license: "LGPL-2.1-or-later", sourceURL: "https://gitlab.gnome.org/GNOME/libadwaita"},
	{name: "gotk4", version: "0.4.1", license: "MPL-2.0", sourceURL: "https://github.com/diamondburned/gotk4"},
	{name: "gotk4-adwaita", version: "d4aecaa0ff32", license: "MPL-2.0", sourceURL: "https://github.com/diamondburned/gotk4-adwaita"},
	{name: "modernc.org/sqlite", version: "1.56.0", license: "BSD-3-Clause", copyright: "Copyright © 2017 The Sqlite Authors", sourceURL: "https://gitlab.com/cznic/sqlite"},
	{name: "google/uuid", version: "1.6.0", license: "BSD-3-Clause", copyright: "Copyright © 2009, 2014 Google Inc.", sourceURL: "https://github.com/google/uuid"},
	{name: "gorilla/websocket", version: "1.5.3", license: "BSD-2-Clause", copyright: "Copyright © 2013 The Gorilla WebSocket Authors", sourceURL: "https://github.com/gorilla/websocket"},
	{name: "pressly/goose", version: "3.27.3", license: "MIT", copyright: "Copyright © goose contributors", sourceURL: "https://github.com/pressly/goose"},
	{name: "Uber Fx", version: "1.24.0", license: "MIT", copyright: "Copyright © 2016–2018 Uber Technologies, Inc.", sourceURL: "https://github.com/uber-go/fx"},
}

func newSettingsButton(t i18n.T, showSettings func()) *gtk.Button {
	button := gtk.NewButtonFromIconName("preferences-system-symbolic")
	button.SetTooltipText(t.Settings)
	button.ConnectClicked(func() {
		if showSettings != nil {
			showSettings()
		}
	})
	return button
}

func newAboutButton(t i18n.T, parent gtk.Widgetter) *gtk.Button {
	button := gtk.NewButtonFromIconName("help-about-symbolic")
	button.SetTooltipText(t.AboutProgram)
	var dialog *adw.AboutDialog
	button.ConnectClicked(func() {
		if dialog == nil {
			dialog = newAboutDialog()
		}
		dialog.Present(parent)
	})
	return button
}

func newAboutDialog() *adw.AboutDialog {
	dialog := adw.NewAboutDialog()
	dialog.SetApplicationIcon(metadata.Current.ID)
	dialog.SetApplicationName(metadata.Current.Name)
	dialog.SetVersion(metadata.Current.Version)
	dialog.SetDeveloperName(metadata.Current.Developer.Name)
	dialog.SetDevelopers([]string{metadata.Current.Developer.Name})
	dialog.SetComments(localizedDescription())
	dialog.SetWebsite(metadata.Current.Homepage)
	dialog.SetIssueURL(metadata.Current.Homepage + "/issues")
	dialog.SetCopyright(fmt.Sprintf("Copyright © 2026 %s", metadata.Current.Developer.Name))
	dialog.SetLicenseType(gtk.LicenseMITX11)

	for _, component := range openSourceComponents {
		title := component.name
		if component.version != "" {
			title += " " + component.version
		}
		dialog.AddLegalSection(title, component.copyright, gtk.LicenseCustom,
			fmt.Sprintf("License: %s\nSource: %s", component.license, component.sourceURL))
	}
	return dialog
}

func localizedDescription() string {
	switch i18n.GetLanguage() {
	case i18n.LangRU:
		return metadata.Current.Description.Russian
	default:
		return metadata.Current.Description.English
	}
}
