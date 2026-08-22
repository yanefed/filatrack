package spools

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newSpoolDryingSection(t i18n.T, status service.DryingStatus, statusErr error) *gtk.Box {
	section := gtk.NewBox(gtk.OrientationVertical, 4)
	section.SetMarginStart(14)
	section.SetMarginEnd(14)

	separator := gtk.NewSeparator(gtk.OrientationHorizontal)
	separator.SetMarginBottom(4)
	section.Append(separator)

	lastText := t.NeverDried
	if status.LastSession != nil {
		lastText = fmt.Sprintf(t.LastDryingFmt, status.LastSession.DriedAt.Format("02.01.2006"))
	}
	lastLabel := dryingCaption(lastText)
	section.Append(lastLabel)

	stateLabel := dryingCaption(dryingStateText(t, status, statusErr))
	if status.NeedsDrying {
		stateLabel.RemoveCSSClass("dim-label")
		stateLabel.AddCSSClass("drying-warning")
	}
	section.Append(stateLabel)

	return section
}

func dryingCaption(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.AddCSSClass("caption")
	label.AddCSSClass("dim-label")
	label.SetHAlign(gtk.AlignStart)
	label.SetWrap(true)
	return label
}

func dryingStateText(t i18n.T, status service.DryingStatus, statusErr error) string {
	if statusErr != nil {
		return fmt.Sprintf("%s: %v", t.ErrorPrefix, statusErr)
	}
	if status.Profile == nil {
		return t.NoDryingProfile
	}
	if status.LastSession == nil {
		return t.DryingRequired
	}
	if status.NeedsDrying {
		if status.DaysUntilNext < 0 {
			return fmt.Sprintf(t.DryingOverdueFmt, -status.DaysUntilNext)
		}
		return t.DryingRequired
	}
	if status.DaysUntilNext == 0 {
		return t.DryingDueToday
	}
	return fmt.Sprintf(t.NextDryingFmt, status.DaysUntilNext)
}

func showDrySpoolDialog(ctx context.Context, parent *gtk.Window, dryingSvc *service.DryingService, spool domain.Spool, profile *domain.DryingProfile, onSave func()) {
	t := i18n.L()
	dialog := shared.NewModalWindow(parent)
	dialog.SetTitle(fmt.Sprintf("%s", t.DrySpool))
	dialog.SetDefaultSize(360, 240)

	box := widgets.NewDialogContent(8)

	spoolLabel := gtk.NewLabel(fmt.Sprintf("%s — %s %s (%s)", spool.DisplayCode, spool.Manufacturer, spool.Color, spool.Material))
	spoolLabel.AddCSSClass("heading")
	spoolLabel.SetHAlign(gtk.AlignStart)
	spoolLabel.SetWrap(true)
	box.Append(spoolLabel)

	parametersGrid := gtk.NewGrid()
	parametersGrid.SetColumnSpacing(10)
	parametersGrid.SetRowSpacing(8)

	temperatureLabel := gtk.NewLabel(t.TemperatureC)
	temperatureLabel.SetHAlign(gtk.AlignEnd)
	temperatureEntry := gtk.NewEntry()
	temperatureEntry.SetPlaceholderText(t.TemperatureC)
	temperatureEntry.SetHExpand(true)
	parametersGrid.Attach(temperatureLabel, 0, 0, 1, 1)
	parametersGrid.Attach(temperatureEntry, 1, 0, 1, 1)

	durationLabel := gtk.NewLabel(t.DurationHours)
	durationLabel.SetHAlign(gtk.AlignEnd)
	durationEntry := gtk.NewEntry()
	durationEntry.SetPlaceholderText(t.DurationHours)
	durationEntry.SetHExpand(true)
	parametersGrid.Attach(durationLabel, 0, 1, 1, 1)
	parametersGrid.Attach(durationEntry, 1, 1, 1, 1)
	box.Append(parametersGrid)

	if profile != nil {
		temperatureEntry.SetText(strconv.Itoa(profile.TemperatureC))
		durationEntry.SetText(strconv.FormatFloat(profile.DurationHours, 'f', -1, 64))
	}

	errorLabel := gtk.NewLabel("")
	errorLabel.AddCSSClass("error")
	errorLabel.SetHAlign(gtk.AlignStart)
	errorLabel.SetWrap(true)
	box.Append(errorLabel)

	actions := widgets.NewActions(8, gtk.AlignEnd)
	cancelBtn := gtk.NewButtonWithLabel(t.Cancel)
	cancelBtn.ConnectClicked(dialog.Destroy)
	actions.Append(cancelBtn)

	saveBtn := gtk.NewButtonWithLabel(t.Save)
	saveBtn.AddCSSClass("suggested-action")
	saveBtn.ConnectClicked(func() {
		temperature, err := strconv.Atoi(temperatureEntry.Text())
		if err != nil {
			errorLabel.SetText(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			return
		}
		duration, err := strconv.ParseFloat(durationEntry.Text(), 64)
		if err != nil {
			errorLabel.SetText(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			return
		}

		session := &domain.DryingSession{
			SpoolID:       spool.ID,
			TemperatureC:  temperature,
			DurationHours: duration,
		}
		if err := dryingSvc.Create(ctx, session); err != nil {
			errorLabel.SetText(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			return
		}

		dialog.Destroy()
		if onSave != nil {
			onSave()
		}
	})
	actions.Append(saveBtn)
	box.Append(actions)

	dialog.SetChild(box)
	dialog.SetVisible(true)
}
