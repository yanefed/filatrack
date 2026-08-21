package ui

import (
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens"
	"awesomeProject/packaging/metadata"
	"context"
	"log/slog"
	"os"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// App содержит зависимости для GTK-интерфейса.
type App struct {
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
	ctx, cancel := context.WithCancel(context.Background())

	installAppCSS()

	window := adw.NewApplicationWindow(&app.Application)
	window.SetTitle(metadata.Current.Name)
	window.SetDefaultSize(1000, 700)

	refreshActive := func(bool) {}
	refreshLive := func() {}
	activePage := "dashboard"
	var currentContent *adw.ToolbarView
	var currentStack *gtk.Stack
	var visibleChildHandle coreglib.SignalHandle
	var rebuildUI func()
	rebuildUI = func() {
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

		stack := gtk.NewStack()
		stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

		refreshDashboard := func() {}
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
			refreshPrintsLive()
			refreshPrintersLive()
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
		refreshSettingsDependencies := func() { markDirty("spools", "drying") }
		refreshModelDependencies := func() { markDirty("dashboard", "printers") }

		dashboardPage, refreshDashboard := screens.NewDashboard(ctx, a.SpoolService, a.PrintService)
		stack.AddTitled(dashboardPage, "dashboard", t.Dashboard)

		spoolsPage, refreshSpools := screens.NewSpoolsScreen(ctx, a.SpoolService, a.SettingsService, refreshSpoolDependencies)
		stack.AddTitled(spoolsPage, "spools", t.Spools)

		printsPage, refreshPrints, refreshPrintsLive := screens.NewPrintsScreen(ctx, a.PrintService, a.PrinterService, a.SpoolService, refreshPrintDependencies)
		stack.AddTitled(printsPage, "prints", t.Prints)

		modelsPage, refreshModels := screens.NewModelsScreen(ctx, a.PrinterService, a.SpoolService, a.SettingsService, refreshModelDependencies)
		stack.AddTitled(modelsPage, "models", t.Models)

		dryingPage, refreshDrying := screens.NewDryingScreen(ctx, a.DryingService, a.SpoolService, refreshDryingDependencies)
		stack.AddTitled(dryingPage, "drying", t.Drying)

		printersPage, refreshPrinters, refreshPrintersLive := screens.NewPrintersScreen(ctx, a.PrinterService, a.SpoolService, refreshPrinterDependencies)
		stack.AddTitled(printersPage, "printers", t.Printers)

		settingsPage, refreshSettings := screens.NewSettingsScreen(ctx, a.SettingsService, refreshSettingsDependencies, rebuildUI)
		stack.AddNamed(settingsPage, "settings")

		sidebar := gtk.NewStackSidebar()
		sidebar.SetStack(stack)
		sidebar.SetSizeRequest(200, -1)

		splitView := gtk.NewBox(gtk.OrientationHorizontal, 0)
		splitView.Append(sidebar)

		separator := gtk.NewSeparator(gtk.OrientationVertical)
		splitView.Append(separator)

		stack.SetHExpand(true)
		stack.SetVExpand(true)
		splitView.Append(stack)

		headerBar := adw.NewHeaderBar()
		headerBar.SetShowEndTitleButtons(true)
		headerBar.PackEnd(newAboutButton(t, window))
		headerBar.PackEnd(newSettingsButton(t, func() {
			stack.SetVisibleChildName("settings")
		}))

		toolbarView := adw.NewToolbarView()
		toolbarView.AddTopBar(headerBar)
		toolbarView.SetContent(splitView)

		window.SetContent(toolbarView)
		currentContent = toolbarView
		currentStack = stack
		stack.SetVisibleChildName(activePage)
		visibleChildHandle = stack.NotifyProperty("visible-child-name", func() {
			activePage = stack.VisibleChildName()
			refreshActive(false)
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
	window.ConnectDestroy(func() {
		cancel()
		glib.SourceRemove(refreshTimer)
		glib.SourceRemove(liveTimer)
		refreshActive = func(bool) {}
		refreshLive = func() {}
	})
}

func installAppCSS() {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}
	provider := gtk.NewCSSProvider()
	provider.LoadFromString(`
.printer-status-dot {
	min-width: 12px;
	min-height: 12px;
	border-radius: 999px;
}
.printer-status-dot.online {
	background: #2ec27e;
}
.printer-status-dot.offline {
	background: #e01b24;
}
`)
	gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}
