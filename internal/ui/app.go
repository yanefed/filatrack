package ui

import (
	"context"
	"os"

	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
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
	app := adw.NewApplication("com.filatrack.app", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() {
		a.activate(app)
	})
	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
	return nil
}

func (a *App) activate(app *adw.Application) {
	ctx := context.Background()

	installAppCSS()

	window := adw.NewApplicationWindow(&app.Application)
	window.SetTitle("FilaTrack")
	window.SetDefaultSize(1000, 700)

	refreshAll := func() {}
	activePage := "dashboard"
	var rebuildUI func()
	rebuildUI = func() {
		t := i18n.L()

		stack := gtk.NewStack()
		stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

		refreshDashboard := func() {}
		refreshSpools := func() {}
		refreshPrints := func() {}
		refreshDrying := func() {}
		refreshPrinters := func() {}
		refreshSettings := func() {}
		refreshAll = func() {
			refreshDashboard()
			refreshSpools()
			refreshPrints()
			refreshDrying()
			refreshPrinters()
			refreshSettings()
		}

		dashboardPage, refreshDashboard := screens.NewDashboard(ctx, a.SpoolService, a.PrintService, a.DryingService)
		stack.AddTitled(dashboardPage, "dashboard", t.Dashboard)

		spoolsPage, refreshSpools := screens.NewSpoolsScreen(ctx, a.SpoolService, refreshAll)
		stack.AddTitled(spoolsPage, "spools", t.Spools)

		printsPage, refreshPrints := screens.NewPrintsScreen(ctx, a.PrintService, a.SpoolService, refreshAll)
		stack.AddTitled(printsPage, "prints", t.Prints)

		dryingPage, refreshDrying := screens.NewDryingScreen(ctx, a.DryingService, a.SpoolService, refreshAll)
		stack.AddTitled(dryingPage, "drying", t.Drying)

		printersPage, refreshPrinters := screens.NewPrintersScreen(ctx, a.PrinterService, a.SpoolService, refreshAll)
		stack.AddTitled(printersPage, "printers", t.Printers)

		settingsPage, refreshSettings := screens.NewSettingsScreen(ctx, a.SettingsService, refreshAll, rebuildUI)
		stack.AddTitled(settingsPage, "settings", t.Settings)

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

		toolbarView := adw.NewToolbarView()
		toolbarView.AddTopBar(headerBar)
		toolbarView.SetContent(splitView)

		window.SetContent(toolbarView)
		stack.SetVisibleChildName(activePage)
		stack.NotifyProperty("visible-child-name", func() {
			activePage = stack.VisibleChildName()
			refreshAll()
		})
	}

	rebuildUI()
	window.Show()

	// Периодическое обновление данных каждые 5 минут
	glib.TimeoutSecondsAdd(300, func() bool {
		refreshAll()
		return true
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
