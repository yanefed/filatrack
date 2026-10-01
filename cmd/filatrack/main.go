package main

import (
	"awesomeProject/internal/config"
	"awesomeProject/internal/database"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/moonraker"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

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
	configureBundleEnvironment()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	runApplication(appOptions()...)
}

func appOptions() []fx.Option {
	return []fx.Option{
		fx.Provide(
			provideLogger,
			config.Default,
			newDatabaseManager,
			func(m *database.Manager) repository.SpoolRepository { return m.Spools() },
			func(m *database.Manager) repository.PrintJobRepository { return m.Prints() },
			func(m *database.Manager) repository.DryingSessionRepository { return m.Dryings() },
			func(m *database.Manager) repository.SettingsRepository { return m.Settings() },
			func(m *database.Manager) repository.PrinterRepository { return m.Printers() },
			func(m *database.Manager) repository.PrinterModelRepository { return m.Models() },
			service.NewSpoolService,
			service.NewPrintService,
			service.NewDryingService,
			service.NewPrinterServiceWithLogger,
			fx.Annotate(
				service.NewSettingsService,
				fx.As(new(languageProvider)),
				fx.As(fx.Self()),
			),
			fx.Annotate(newUIApp, fx.As(new(applicationRunner))),
		),
		fx.Invoke(configurePrinterConnectionChecker),
		fx.Invoke(configurePrinterJobClient),
		fx.Invoke(configurePrinterModelCache),
		fx.Invoke(startPrinterMonitors),
	}
}

func newDatabaseManager(lifecycle fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (*database.Manager, error) {
	m, err := database.New(context.Background(), cfg, logger)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error { return m.Close() }})
	return m, nil
}

func newUIApp(spools *service.SpoolService, prints *service.PrintService, dryings *service.DryingService, settings *service.SettingsService, printers *service.PrinterService, db *database.Manager) *ui.App {
	app := ui.NewApp(spools, prints, dryings, settings, printers)
	app.Database = db
	return app
}

func provideLogger() *slog.Logger {
	return slog.Default()
}

func startApplication(options ...fx.Option) {
	var app applicationRunner
	var settingsSvc languageProvider
	options = append(options, fx.Populate(&app, &settingsSvc))

	fxApp := fx.New(options...)
	startCtx, cancelStart := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStart()
	if err := fxApp.Start(startCtx); err != nil {
		slog.Error("starting application services", "error", err)
		exitProcess(1)
		return
	}

	runApp(app, settingsSvc)

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()
	if err := fxApp.Stop(stopCtx); err != nil {
		slog.Error("stopping application services", "error", err)
		exitProcess(1)
	}
}

func newDB(cfg *config.Config) (*sqlite.DB, error) {
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating application data directory: %w", err)
		}
	}
	db, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("migrating database: %w; closing database: %v", err, closeErr)
		}
		return nil, err
	}
	return db, nil
}

func startPrinterMonitors(lifecycle fx.Lifecycle, cfg *config.Config, printerSvc *service.PrinterService, db *database.Manager) {
	runner := moonraker.NewRunner(printerSvc, printerSvc, moonraker.WebSocketDialer{})
	runner.Metadata = moonraker.HTTPMetadataFetcher{CacheDir: filepath.Join(cfg.DataDir, "cache", "gcode-previews")}
	runner.History = printerSvc
	db.SetActivityControl(runner.Stop, runner.Start)
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return runner.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return runner.Stop()
		},
	})
}

func configurePrinterConnectionChecker(printerSvc *service.PrinterService) {
	printerSvc.SetConnectionChecker(moonraker.HTTPMetadataFetcher{})
}

func configurePrinterJobClient(cfg *config.Config, printerSvc *service.PrinterService) {
	printerSvc.SetJobClient(moonraker.HTTPControlClient{CacheDir: filepath.Join(cfg.DataDir, "cache", "gcode-previews")})
}

func configurePrinterModelCache(printerSvc *service.PrinterService, models repository.PrinterModelRepository) {
	printerSvc.SetModelCache(models)
}

func runApp(app applicationRunner, settingsSvc languageProvider) {
	lang := settingsSvc.GetLanguage(context.Background())
	i18n.SetLanguage(lang)

	if err := app.Run(); err != nil {
		slog.Error("application error", "error", err)
		exitProcess(1)
		return
	}
}
