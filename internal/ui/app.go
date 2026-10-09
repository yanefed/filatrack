package ui

import (
	"context"
	"github.com/yanefed/filatrack/internal/database"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
	"github.com/yanefed/filatrack/internal/ui/layout"
	"github.com/yanefed/filatrack/internal/ui/screens/dashboard"
	"github.com/yanefed/filatrack/internal/ui/screens/drying"
	"github.com/yanefed/filatrack/internal/ui/screens/models"
	"github.com/yanefed/filatrack/internal/ui/screens/printers"
	"github.com/yanefed/filatrack/internal/ui/screens/prints"
	"github.com/yanefed/filatrack/internal/ui/screens/settings"
	"github.com/yanefed/filatrack/internal/ui/screens/spools"
	"github.com/yanefed/filatrack/packaging/metadata"
	"log/slog"
	"os"
	"runtime"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// App содержит зависимости для GTK-интерфейса.
type App struct {
	Database        *database.Manager
	SpoolService    *service.SpoolService
	PrintService    *service.PrintService
	DryingService   *service.DryingService
	SettingsService *service.SettingsService
	PrinterService  *service.PrinterService
}

func NewApp(spools *service.SpoolService, prints *service.PrintService, dryings *service.DryingService, settings *service.SettingsService, printers *service.PrinterService) *App {
	return &App{
		SpoolService:    spools,
		PrintService:    prints,
		DryingService:   dryings,
		SettingsService: settings,
		PrinterService:  printers,
	}
}

// Run запускает GTK4-приложение.
func (a *App) Run() error {
	if err := i18n.ConfigureToolkitLanguage(i18n.GetLanguage()); err != nil {
		slog.Warn("configuring GTK language", "language", i18n.GetLanguage(), "error", err)
	}
	app := adw.NewApplication(metadata.Current.ID, gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		a.activate(app)
	})
	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
	return nil
}

func (a *App) activate(app *adw.Application) {
	view := layout.Load("main-window")

	ctx, cancel := context.WithCancel(context.Background())

	installAppCSS()

	window := layout.Configure[*adw.ApplicationWindow](view, "window", func(w *adw.ApplicationWindow) { w.SetApplication(&app.Application) })
	window.SetTitle(metadata.Current.Name)

	parentWindow := &window.ApplicationWindow.Window

	refreshActive := func(bool) {}
	refreshLive := func() {}
	activePage := "dashboard"
	var currentContent *adw.ToolbarView
	var currentStack *gtk.Stack
	var visibleChildHandle coreglib.SignalHandle
	var rebuildUI func()
	rebuildUI = func() {
		view := layout.Load("main-layout")

		if err := i18n.ConfigureToolkitLanguage(i18n.GetLanguage()); err != nil {
			slog.Warn("updating GTK language", "language", i18n.GetLanguage(), "error", err)
		}
		// Signal closures hold their Go captures as long as their GObject lives.
		// Release the previous root before building the translated replacement.
		if currentStack != nil {
			currentStack.HandlerDisconnect(visibleChildHandle)
			currentStack = nil
		}
		if currentContent != nil {
			window.SetContent(nil)
			currentContent = nil
		}

		t := i18n.L()

		stack := layout.Get[*gtk.Stack](view, "stack")

		refreshDashboard := func() {}
		refreshDashboardLive := func() {}
		refreshSpools := func() {}
		refreshPrints := func() {}
		refreshModels := func() {}
		refreshDrying := func() {}
		refreshPrinters := func() {}
		refreshPrintsLive := func() {}
		refreshPrintersLive := func() {}
		refreshSettings := func() {}
		dirty := map[string]bool{
			"dashboard": false,
			"spools":    false,
			"prints":    false,
			"models":    false,
			"drying":    false,
			"printers":  false,
			"settings":  false,
		}
		refreshLive = func() {
			switch activePage {
			case "prints":
				refreshPrintsLive()
			case "printers":
				refreshPrintersLive()
			case "dashboard":
				refreshDashboardLive()
			}
		}

		refreshPage := func(page string) {
			switch page {
			case "dashboard":
				refreshDashboard()
			case "spools":
				refreshSpools()
			case "prints":
				refreshPrints()
			case "models":
				refreshModels()
			case "drying":
				refreshDrying()
			case "printers":
				refreshPrinters()
			case "settings":
				refreshSettings()
			}
			dirty[page] = false
		}
		refreshActive = func(force bool) {
			if force || dirty[activePage] {
				refreshPage(activePage)
			}
		}
		markDirty := func(pages ...string) {
			for _, page := range pages {
				if page == activePage {
					refreshPage(page)
					continue
				}
				dirty[page] = true
			}
		}
		refreshPrintDependencies := func() { markDirty("dashboard", "spools", "printers") }
		refreshSpoolDependencies := func() { markDirty("dashboard", "prints", "models", "drying", "printers", "settings") }
		refreshPrinterDependencies := func() { markDirty("dashboard", "prints") }
		refreshDryingDependencies := func() { markDirty("dashboard", "spools") }
		refreshSettingsDependencies := func() { markDirty("spools", "drying", "prints") }
		refreshModelDependencies := func() { markDirty("dashboard", "printers") }

		dashboardPage, refreshDashboard, refreshDashboardLive := dashboard.NewDashboard(ctx, a.SpoolService, a.PrintService, a.PrinterService, layout.Scope(view, "dashboard"))
		stack.Page(dashboardPage).SetTitle(t.Dashboard)

		spoolsPage, refreshSpools, openAddSpoolDialog := spools.NewSpoolsScreen(ctx, parentWindow, a.SpoolService, a.DryingService, a.SettingsService, refreshSpoolDependencies, layout.Scope(view, "spools"))
		stack.Page(spoolsPage).SetTitle(t.Spools)

		printsPage, refreshPrints, refreshPrintsLive, refreshHistoryButton, printsPrinterFilter := prints.NewPrintsScreen(ctx, parentWindow, a.PrintService, a.PrinterService, a.SpoolService, refreshPrintDependencies, layout.Scope(view, "prints"))
		stack.Page(printsPage).SetTitle(t.Prints)

		modelsPage, refreshModels, addModelButton, refreshModelsButton := models.NewModelsScreen(ctx, parentWindow, a.PrinterService, a.SpoolService, a.SettingsService, refreshModelDependencies, layout.Scope(view, "models"))
		stack.Page(modelsPage).SetTitle(t.Models)

		dryingPage, refreshDrying, addDryingButton := drying.NewDryingScreen(ctx, parentWindow, a.DryingService, a.SpoolService, refreshDryingDependencies, layout.Scope(view, "drying"))
		stack.Page(dryingPage).SetTitle(t.Drying)

		printersPage, refreshPrinters, refreshPrintersLive, addPrinterButton := printers.NewPrintersScreen(ctx, parentWindow, a.PrinterService, a.SpoolService, refreshPrinterDependencies, layout.Scope(view, "printers"))
		stack.Page(printersPage).SetTitle(t.Printers)

		settingsPage, refreshSettings := settings.NewSettingsScreen(ctx, parentWindow, a.SettingsService, refreshSettingsDependencies, rebuildUI, settings.DatabaseOptions{Manager: a.Database, OnSwitched: app.Quit, View: layout.Scope(view, "settings")})
		_ = settingsPage

		addSpoolButton := layout.Get[*gtk.Button](view, "addSpoolButton")
		addSpoolButton.SetTooltipText(t.AddSpoolBtn)
		addSpoolButton.ConnectClicked(openAddSpoolDialog)
		_ = newAboutButton(t, window, layout.Scope(view, "headerBar_slot_1"))
		_ = newSettingsButton(t, func() {
			stack.SetVisibleChildName("settings")
		}, layout.Scope(view, "headerBar_slot_2"))

		toolbarView := layout.Get[*adw.ToolbarView](view, "toolbarView")

		window.SetContent(toolbarView)
		currentContent = toolbarView
		currentStack = stack
		stack.SetVisibleChildName(activePage)
		updateHeaderActions := func() {
			addSpoolButton.SetVisible(stackShowsPage(stack, spoolsPage))
			refreshHistoryButton.SetVisible(stackShowsPage(stack, printsPage))
			printsPrinterFilter.SetVisible(stackShowsPage(stack, printsPage))
			addModelButton.SetVisible(stackShowsPage(stack, modelsPage))
			refreshModelsButton.SetVisible(stackShowsPage(stack, modelsPage))
			addDryingButton.SetVisible(stackShowsPage(stack, dryingPage))
			addPrinterButton.SetVisible(stackShowsPage(stack, printersPage))
		}
		updateHeaderActions()
		visibleChildHandle = stack.NotifyProperty("visible-child", func() {
			activePage = stack.VisibleChildName()
			updateHeaderActions()
			refreshActive(false)
			refreshLive()
		})
	}

	rebuildUI()
	window.SetVisible(true)

	// Периодическое обновление данных каждые 5 минут
	refreshTimer := glib.TimeoutSecondsAdd(300, func() bool {
		refreshActive(true)
		return true
	})
	liveTimer := glib.TimeoutSecondsAdd(2, func() bool {
		refreshLive()
		return true
	})
	memoryTimer := startMemoryDiagnostics()
	window.ConnectDestroy(func() {
		cancel()
		glib.SourceRemove(refreshTimer)
		glib.SourceRemove(liveTimer)
		if memoryTimer != 0 {
			glib.SourceRemove(memoryTimer)
		}
		refreshActive = func(bool) {}
		refreshLive = func() {}
	})
}

func stackShowsPage(stack *gtk.Stack, page gtk.Widgetter) bool {
	visiblePage := stack.VisibleChild()
	return visiblePage != nil && coreglib.BaseObject(visiblePage).Native() == coreglib.BaseObject(page).Native()
}

func installAppCSS() {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}
	provider := gtk.NewCSSProvider()
	css := layout.CSS("app")
	if runtime.GOOS == "darwin" {
		css += layout.CSS("macos")
	}
	provider.LoadFromString(css)
	gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}
