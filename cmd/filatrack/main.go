package main

import (
	"context"
	"log/slog"
	"os"

	"awesomeProject/internal/config"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"awesomeProject/internal/ui"

	"go.uber.org/fx"
	_ "modernc.org/sqlite"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	fx.New(
		fx.Provide(
			config.Default,
			newDB,
			sqlite.NewSpoolRepo,
			sqlite.NewPrintJobRepo,
			sqlite.NewDryingRepo,
			sqlite.NewSettingsRepo,
			service.NewSpoolService,
			service.NewPrintService,
			service.NewDryingService,
			service.NewSettingsService,
			ui.NewApp,
		),
		fx.Invoke(runApp),
	).Run()
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

func runApp(app *ui.App, settingsSvc *service.SettingsService) {
	lang := settingsSvc.GetLanguage(context.Background())
	i18n.SetLanguage(lang)

	if err := app.Run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
