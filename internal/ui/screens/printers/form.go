package printers

import (
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
)

type printerForm struct {
	root        *adw.PreferencesGroup
	nameEntry   *adw.EntryRow
	urlEntry    *adw.EntryRow
	apiKeyEntry *adw.PasswordEntryRow
}

func newPrinterForm(t i18n.T, printer *domain.Printer, views ...*layout.View) *printerForm {
	view := layout.Use("printer-form", views...)

	form := &printerForm{root: layout.Get[*adw.PreferencesGroup](view, "form_root")}
	form.nameEntry = layout.Get[*adw.EntryRow](view, "form_nameEntry")
	form.nameEntry.SetTitle(t.PrinterName)

	form.urlEntry = layout.Get[*adw.EntryRow](view, "form_urlEntry")
	form.urlEntry.SetTitle(t.MoonrakerURL)

	form.apiKeyEntry = layout.Get[*adw.PasswordEntryRow](view, "form_apiKeyEntry")
	form.apiKeyEntry.SetTitle(t.APIKey)

	if printer != nil {
		form.nameEntry.SetText(printer.Name)
		form.urlEntry.SetText(printer.MoonrakerURL)
		form.apiKeyEntry.SetText(printer.APIKey)
	}
	return form
}

func (f *printerForm) Printer(base domain.Printer) domain.Printer {
	base.Name = strings.TrimSpace(f.nameEntry.Text())
	base.MoonrakerURL = strings.TrimSpace(f.urlEntry.Text())
	base.APIKey = strings.TrimSpace(f.apiKeyEntry.Text())
	return base
}

func (f *printerForm) Valid() bool {
	name := strings.TrimSpace(f.nameEntry.Text())
	url := strings.TrimSpace(f.urlEntry.Text())
	return name != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"))
}
