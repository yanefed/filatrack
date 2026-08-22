package printers

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type printerForm struct {
	root        *gtk.Box
	nameEntry   *gtk.Entry
	urlEntry    *gtk.Entry
	apiKeyEntry *gtk.Entry
}

func newPrinterForm(t i18n.T) *printerForm {
	form := &printerForm{root: gtk.NewBox(gtk.OrientationVertical, 6)}
	form.root.SetMarginTop(8)
	form.nameEntry = gtk.NewEntry()
	form.nameEntry.SetPlaceholderText(t.PrinterName)
	form.root.Append(form.nameEntry)
	form.urlEntry = gtk.NewEntry()
	form.urlEntry.SetPlaceholderText(t.MoonrakerURL)
	form.root.Append(form.urlEntry)
	form.apiKeyEntry = gtk.NewEntry()
	form.apiKeyEntry.SetPlaceholderText(t.APIKey)
	form.apiKeyEntry.SetVisibility(false)
	form.root.Append(form.apiKeyEntry)
	return form
}

func (f *printerForm) Printer() domain.Printer {
	return domain.Printer{
		Name:         f.nameEntry.Text(),
		MoonrakerURL: f.urlEntry.Text(),
		APIKey:       strings.TrimSpace(f.apiKeyEntry.Text()),
	}
}

func (f *printerForm) Reset() {
	f.nameEntry.SetText("")
	f.urlEntry.SetText("")
	f.apiKeyEntry.SetText("")
}

func newPrinterRow(t i18n.T, printer domain.Printer, spools []domain.Spool, selected uint, onSelected func(uint), onSave func(uint), onEdit func()) (*gtk.Box, *gtk.Box) {
	row := gtk.NewBox(gtk.OrientationVertical, 4)
	row.SetMarginTop(6)
	row.SetMarginBottom(8)

	statusSlot := gtk.NewBox(gtk.OrientationVertical, 0)
	statusSlot.Append(widgets.NewPrinterStatus(t, printer))
	row.Append(statusSlot)

	assignRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	assignLabel := gtk.NewLabel(t.ActiveSpool)
	assignLabel.SetHAlign(gtk.AlignStart)
	assignRow.Append(assignLabel)
	dropdown := gtk.NewDropDown(gtk.NewStringList(shared.SpoolChoiceNames(t, spools)), nil)
	dropdown.SetSelected(selected)
	dropdown.NotifyProperty("selected", func() { onSelected(dropdown.Selected()) })
	assignRow.Append(dropdown)

	saveBtn := gtk.NewButtonWithLabel(t.SaveAssignment)
	saveBtn.ConnectClicked(func() { onSave(dropdown.Selected()) })
	assignRow.Append(saveBtn)
	editBtn := gtk.NewButtonWithLabel(t.Edit)
	editBtn.ConnectClicked(onEdit)
	assignRow.Append(editBtn)
	row.Append(assignRow)
	return row, statusSlot
}
