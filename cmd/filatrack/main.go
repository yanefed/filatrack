package main

import (
	"context"
	"log/slog"
	"os"

	"awesomeProject/internal/config"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/moonraker"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui"

	"go.uber.org/fx"
	_ "modernc.org/sqlite"
)

type applicationRunner interface {
	Run() error
}

type languageProvider interface {
	GetLanguage(context.Context) i18n.Language
}

var (
	runApplication = startApplication
	exitProcess    = os.Exit
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	runApplication(appOptions()...)
}

func appOptions() []fx.Option {
	return []fx.Option{
		fx.Provide(
			config.Default,
			newDB,
			sqlite.NewSpoolRepo,
			sqlite.NewPrintJobRepo,
			sqlite.NewDryingRepo,
			sqlite.NewSettingsRepo,
			sqlite.NewPrinterRepo,
			service.NewSpoolService,
			service.NewPrintService,
			service.NewDryingService,
			service.NewPrinterService,
			fx.Annotate(
				service.NewSettingsService,
				fx.As(new(languageProvider)),
				fx.As(fx.Self()),
			),
			fx.Annotate(ui.NewApp, fx.As(new(applicationRunner))),
		),
		fx.Invoke(configurePrinterConnectionChecker),
		fx.Invoke(startPrinterMonitors),
		fx.Invoke(runApp),
	}
}

func startApplication(options ...fx.Option) {
	fx.New(options...).Run()
}

func newDB(cfg *config.Config) (*sqlite.DB, error) {
	db, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func startPrinterMonitors(lifecycle fx.Lifecycle, printerSvc *service.PrinterService) {
	runner := moonraker.NewRunner(printerSvc, printerSvc, moonraker.WebSocketDialer{})
	runner.Metadata = moonraker.HTTPMetadataFetcher{}
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return runner.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return runner.Stop(ctx)
		},
	})
}

func configurePrinterConnectionChecker(printerSvc *service.PrinterService) {
	printerSvc.SetConnectionChecker(moonraker.HTTPMetadataFetcher{})
}

func runApp(app applicationRunner, settingsSvc languageProvider, shutdowner fx.Shutdowner) {
	lang := settingsSvc.GetLanguage(context.Background())
	i18n.SetLanguage(lang)

	if err := app.Run(); err != nil {
		slog.Error("application error", "error", err)
		exitProcess(1)
		return
	}

	if err := shutdowner.Shutdown(); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
