package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
)

type printerForm struct {
	root        *adw.PreferencesGroup
	nameEntry   *adw.EntryRow
	urlEntry    *adw.EntryRow
	apiKeyEntry *adw.PasswordEntryRow
}

func newPrinterForm(t i18n.T, printer *domain.Printer) *printerForm {
	form := &printerForm{root: adw.NewPreferencesGroup()}
	form.nameEntry = adw.NewEntryRow()
	form.nameEntry.SetTitle(t.PrinterName)
	form.root.Add(form.nameEntry)

	form.urlEntry = adw.NewEntryRow()
	form.urlEntry.SetTitle(t.MoonrakerURL)
	form.root.Add(form.urlEntry)

	form.apiKeyEntry = adw.NewPasswordEntryRow()
	form.apiKeyEntry.SetTitle(t.APIKey)
	form.root.Add(form.apiKeyEntry)

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
