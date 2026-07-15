package ui

import (
	"context"
	"os"

	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui/screens"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
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
}

func NewApp(spools *service.SpoolService, prints *service.PrintService, dryings *service.DryingService, settings *service.SettingsService) *App {
	return &App{
		SpoolService:    spools,
		PrintService:    prints,
		DryingService:   dryings,
		SettingsService: settings,
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

	window := adw.NewApplicationWindow(&app.Application)
	window.SetTitle("FilaTrack")
	window.SetDefaultSize(1000, 700)

	t := i18n.L()

	stack := gtk.NewStack()
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	dashboardPage, refreshDashboard := screens.NewDashboard(ctx, a.SpoolService, a.PrintService, a.DryingService)
	stack.AddTitled(dashboardPage, "dashboard", t.Dashboard)

	spoolsPage, refreshSpools := screens.NewSpoolsScreen(ctx, a.SpoolService)
	stack.AddTitled(spoolsPage, "spools", t.Spools)

	printsPage := screens.NewPrintsScreen(ctx, a.PrintService, a.SpoolService, func() {
		refreshSpools()
		refreshDashboard()
	})
	stack.AddTitled(printsPage, "prints", t.Prints)

	dryingPage := screens.NewDryingScreen(ctx, a.DryingService, a.SpoolService, func() {
		refreshDashboard()
	})
	stack.AddTitled(dryingPage, "drying", t.Drying)

	settingsPage := screens.NewSettingsScreen(ctx, a.SettingsService)
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
	window.Show()

	// Периодическое обновление дашборда каждые 5 минут
	glib.TimeoutSecondsAdd(300, func() bool {
		refreshDashboard()
		return true
	})
}
