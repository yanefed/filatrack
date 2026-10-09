package prints

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/layout"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type printListState struct {
	jobs   []domain.PrintJob
	spools []domain.Spool
}

type printRowWidgets struct {
	item      *gtk.ListItem
	picture   *gtk.Picture
	status    *gtk.Image
	header    *gtk.Label
	details   *gtk.Label
	cost      *gtk.Label
	assignRow *gtk.Box
	assignBtn *gtk.Button
}

func newVirtualPrintsList(t i18n.T, ctx context.Context, printSvc *service.PrintService, messageLabel *gtk.Label, views ...*layout.View) (*gtk.ListView, func([]domain.PrintJob, []domain.Spool)) {
	view := layout.Use("print-list", views...)

	state := &printListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*printRowWidgets)
	var activeJobID string
	var activeButton *gtk.Button
	var activeRow *gtk.Box
	chooserModel := gtk.NewStringList(nil)
	chooser := layout.Configure[*gtk.DropDown](view, "chooser", func(w *gtk.DropDown) { w.SetModel(chooserModel) })
	changingSelection := false
	finishChoosing := func() {
		if activeRow != nil && chooser.Parent() != nil {
			activeRow.Remove(chooser)
			if activeButton != nil {
				activeRow.Append(activeButton)
			}
		}
		activeJobID = ""
		activeButton = nil
		activeRow = nil
	}
	chooser.NotifyProperty("selected", func() {
		if changingSelection || activeJobID == "" {
			return
		}
		idx := chooser.Selected()
		if idx == gtk.InvalidListPosition || int(idx) > len(state.spools) {
			return
		}
		spoolID := ""
		if idx > 0 {
			spoolID = state.spools[idx-1].ID
		}
		jobIndex := -1
		for i := range state.jobs {
			if state.jobs[i].ID == activeJobID {
				jobIndex = i
				break
			}
		}
		if jobIndex < 0 {
			finishChoosing()
			return
		}
		job := &state.jobs[jobIndex]
		if spoolID == job.SpoolID {
			finishChoosing()
			return
		}
		if err := printSvc.ChangeSpool(ctx, job.ID, spoolID); err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
			messageLabel.SetVisible(true)
			return
		}
		job.SpoolID = spoolID
		if activeButton != nil {
			activeButton.SetLabel(printSpoolButtonLabel(t, spoolID, state.spools))
		}
		messageLabel.SetVisible(false)
		finishChoosing()
		updated, err := printSvc.GetByID(ctx, job.ID)
		if err != nil {
			messageLabel.SetLabel(fmt.Sprintf("%s: %v", t.ErrorPrefix, err))
			messageLabel.SetVisible(true)
			return
		}
		state.jobs[jobIndex] = *updated
		model.Splice(uint(jobIndex), 1, []string{updated.ID})
	})
	focus := gtk.NewEventControllerFocus()
	focus.ConnectLeave(func() {
		coreglib.IdleAdd(func() {
			if activeRow != nil && !focus.ContainsFocus() {
				finishChoosing()
			}
		})
	})
	chooser.AddController(focus)
	setChooserSpools := func(spools []domain.Spool) {
		chooserModel.Splice(0, chooserModel.NItems(), shared.SpoolChoiceNames(t, spools))
	}

	factory.ConnectSetup(func(object *coreglib.Object) {
		view := layout.Load("print-row")

		item := object.Cast().(*gtk.ListItem)
		item.SetActivatable(false)
		item.SetFocusable(false)
		card := layout.Get[*gtk.Box](view, "card")

		picture := layout.Get[*gtk.Picture](view, "picture")

		status := layout.Get[*gtk.Image](view, "status")

		header := layout.Get[*gtk.Label](view, "header")

		details := layout.Get[*gtk.Label](view, "details")

		cost := layout.Get[*gtk.Label](view, "cost")

		assignRow := layout.Get[*gtk.Box](view, "assignRow")

		assignBtn := layout.Configure[*gtk.Button](view, "assignBtn", func(w *gtk.Button) { w.SetLabel(t.SpoolColon) })
		//assignBtn.AddCSSClass("flat")

		item.SetChild(card)

		itemNative := item.Native()
		rowWidgets := &printRowWidgets{item: item, picture: picture, status: status, header: header, details: details, cost: cost, assignRow: assignRow, assignBtn: assignBtn}
		rows[itemNative] = rowWidgets
		assignBtn.ConnectClicked(func() {
			rowWidgets := rows[itemNative]
			if rowWidgets == nil {
				return
			}
			position := rowWidgets.item.Position()
			if position == gtk.InvalidListPosition || int(position) >= len(state.jobs) {
				return
			}
			if activeRow != nil {
				finishChoosing()
			}
			activeJobID = state.jobs[position].ID
			activeButton = rowWidgets.assignBtn
			activeRow = rowWidgets.assignRow
			changingSelection = true
			chooser.SetSelected(shared.SelectedSpoolIndex(state.jobs[position].SpoolID, state.spools))
			changingSelection = false
			rowWidgets.assignRow.Remove(rowWidgets.assignBtn)
			rowWidgets.assignRow.Append(chooser)
			chooser.GrabFocus()
			coreglib.IdleAdd(func() {
				if activeRow == rowWidgets.assignRow {
					chooser.Activate()
					limitDropDownHeight(chooser, 320)
				}
			})
		})
	})

	factory.ConnectBind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		row := rows[item.Native()]
		position := item.Position()
		if row == nil || int(position) >= len(state.jobs) {
			return
		}
		job := state.jobs[position]
		statusIcon := "object-select-symbolic"
		statusClass := "success"
		switch job.Status {
		case "failed":
			statusIcon = "dialog-error-symbolic"
			statusClass = "error"
		case "cancelled":
			statusIcon = "process-stop-symbolic"
			statusClass = "warning"
		}
		title := job.ModelName
		if title == "" {
			title = job.GcodeFilename
		}
		row.status.SetFromIconName(statusIcon)
		row.status.RemoveCSSClass("success")
		row.status.RemoveCSSClass("error")
		row.status.RemoveCSSClass("warning")
		row.status.AddCSSClass(statusClass)
		row.header.SetLabel(title)
		row.details.SetLabel(printJobUsageText(t, job))
		row.cost.SetLabel(printCostText(t, job))
		widgets.SetPreview(row.picture, job.PreviewImagePath)
		row.assignRow.SetVisible(len(state.spools) > 0)
		if activeButton == row.assignBtn {
			finishChoosing()
		}
		row.assignBtn.SetLabel(printSpoolButtonLabel(t, job.SpoolID, state.spools))
	})
	factory.ConnectUnbind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		if row := rows[item.Native()]; row != nil {
			row.picture.SetPaintable(nil)
			row.picture.SetVisible(false)
		}
	})
	factory.ConnectTeardown(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ListItem)
		delete(rows, item.Native())
	})

	listView := layout.Configure[*gtk.ListView](view, "view", func(w *gtk.ListView) { w.SetModel(selection); w.SetFactory(&factory.ListItemFactory) })

	update := func(jobs []domain.PrintJob, spools []domain.Spool) {
		finishChoosing()
		state.jobs = jobs
		state.spools = spools
		setChooserSpools(spools)
		keys := make([]string, len(jobs))
		for i := range jobs {
			keys[i] = jobs[i].ID
		}
		model.Splice(0, model.NItems(), keys)
	}
	return listView, update
}

func printSpoolButtonLabel(t i18n.T, spoolID string, spools []domain.Spool) string {
	for _, spool := range spools {
		if spool.ID == spoolID {
			return fmt.Sprintf(" %s", shared.SpoolDisplayName(spool))
		}
	}
	return fmt.Sprintf("%s %s", t.SpoolColon, t.UnassignedSpool)
}

func printCostText(t i18n.T, job domain.PrintJob) string {
	cost, known := job.FilamentCost()
	if !known {
		return t.CostNotSet
	}
	estimatePrefix := ""
	if job.CostIsEstimate() {
		estimatePrefix = "≈"
	}
	return fmt.Sprintf(t.PrintCostFmt, estimatePrefix, cost, job.CostCurrency)
}

func limitDropDownHeight(root gtk.Widgetter, maxHeight int) {
	if scrolled, ok := root.(*gtk.ScrolledWindow); ok {
		scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
		scrolled.SetPropagateNaturalHeight(true)
		scrolled.SetMaxContentHeight(maxHeight)
	}
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		limitDropDownHeight(child, maxHeight)
	}
}
