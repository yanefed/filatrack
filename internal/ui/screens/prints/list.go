package prints

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"context"
	"fmt"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

type printListState struct {
	jobs   []domain.PrintJob
	spools []domain.Spool
}

type printRowWidgets struct {
	picture   *gtk.Picture
	header    *gtk.Label
	details   *gtk.Label
	assignRow *gtk.Box
	assignBtn *gtk.Button
}

func newVirtualPrintsList(t i18n.T, ctx context.Context, printSvc *service.PrintService, messageLabel *gtk.Label) (*gtk.ListView, func([]domain.PrintJob, []domain.Spool)) {
	state := &printListState{}
	model := gtk.NewStringList(nil)
	selection := gtk.NewNoSelection(model)
	factory := gtk.NewSignalListItemFactory()
	rows := make(map[uintptr]*printRowWidgets)
	var activeJobID string
	var activeButton *gtk.Button
	var activeRow *gtk.Box
	chooserModel := gtk.NewStringList(nil)
	chooser := gtk.NewDropDown(chooserModel, nil)
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
		item := object.Cast().(*gtk.ListItem)
		item.SetActivatable(false)
		item.SetFocusable(false)
		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.SetMarginTop(6)
		row.SetMarginBottom(6)
		picture := gtk.NewPicture()
		picture.SetSizeRequest(widgets.PreviewWidth, widgets.PreviewHeight)
		picture.SetContentFit(gtk.ContentFitContain)
		picture.SetVisible(false)
		row.Append(picture)
		content := gtk.NewBox(gtk.OrientationVertical, 4)
		content.SetHExpand(true)
		header := gtk.NewLabel("")
		header.SetHAlign(gtk.AlignStart)
		header.SetHExpand(true)
		header.SetEllipsize(pango.EllipsizeEnd)
		header.SetMaxWidthChars(80)
		content.Append(header)
		details := gtk.NewLabel("")
		details.SetHAlign(gtk.AlignStart)
		details.AddCSSClass("dim-label")
		content.Append(details)
		assignRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
		assignBtn := gtk.NewButtonWithLabel(t.SpoolColon)
		assignRow.Append(assignBtn)
		content.Append(assignRow)
		row.Append(content)
		item.SetChild(row)

		rowWidgets := &printRowWidgets{picture: picture, header: header, details: details, assignRow: assignRow, assignBtn: assignBtn}
		rows[item.Native()] = rowWidgets
		assignBtn.ConnectClicked(func() {
			rowWidgets := rows[item.Native()]
			if rowWidgets == nil {
				return
			}
			position := item.Position()
			if position == gtk.InvalidListPosition || int(position) >= len(state.jobs) {
				return
			}
			if activeRow != nil {
				finishChoosing()
			}
			activeJobID = state.jobs[position].ID
			activeButton = assignBtn
			activeRow = rowWidgets.assignRow
			changingSelection = true
			chooser.SetSelected(shared.SelectedSpoolIndex(state.jobs[position].SpoolID, state.spools))
			changingSelection = false
			rowWidgets.assignRow.Remove(assignBtn)
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
		statusIcon := "✅"
		switch job.Status {
		case "failed":
			statusIcon = "❌"
		case "cancelled":
			statusIcon = "🚫"
		}
		title := job.ModelName
		if title == "" {
			title = job.GcodeFilename
		}
		row.header.SetLabel(fmt.Sprintf("%s %s", statusIcon, title))
		row.details.SetLabel(printJobUsageText(t, job))
		texture := widgets.LoadPreviewTexture(job.PreviewImagePath)
		if texture != nil {
			row.picture.SetPaintable(texture)
			row.picture.SetVisible(true)
		} else {
			row.picture.SetPaintable(nil)
			row.picture.SetVisible(false)
		}
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

	view := gtk.NewListView(selection, &factory.ListItemFactory)
	view.SetVExpand(true)
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
	return view, update
}

func printSpoolButtonLabel(t i18n.T, spoolID string, spools []domain.Spool) string {
	for _, spool := range spools {
		if spool.ID == spoolID {
			return fmt.Sprintf(" %s", shared.SpoolDisplayName(spool))
		}
	}
	return fmt.Sprintf("%s %s", t.SpoolColon, t.UnassignedSpool)
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
