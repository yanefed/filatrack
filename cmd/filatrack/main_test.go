package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"awesomeProject/internal/config"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository/sqlite"

	"go.uber.org/fx"
)

type stubApplication struct {
	run func() error
}

func (a stubApplication) Run() error {
	return a.run()
}

type stubLanguageProvider struct {
	language i18n.Language
}

func (p stubLanguageProvider) GetLanguage(context.Context) i18n.Language {
	return p.language
}

func TestMainPassesValidOptionsToApplicationRunner(t *testing.T) {
	originalRunner := runApplication
	originalLogger := slog.Default()
	t.Cleanup(func() {
		runApplication = originalRunner
		slog.SetDefault(originalLogger)
	})

	var captured []fx.Option
	runApplication = func(options ...fx.Option) {
		captured = options
	}

	main()

	if len(captured) == 0 {
		t.Fatal("expected main to pass Fx options to the application runner")
	}
	if err := fx.ValidateApp(captured...); err != nil {
		t.Fatalf("validating application dependency graph: %v", err)
	}
}

func TestAppOptionsBuildAValidDependencyGraph(t *testing.T) {
	if err := fx.ValidateApp(appOptions()...); err != nil {
		t.Fatalf("validating application dependency graph: %v", err)
	}
}

func TestStartApplicationRunsAndStopsFxApp(t *testing.T) {
	var started, stopped, appRan bool

	startApplication(
		fx.NopLogger,
		fx.Provide(
			func() applicationRunner {
				return stubApplication{run: func() error {
					if !started {
						t.Error("expected Fx lifecycle to start before application Run")
					}
					if stopped {
						t.Error("expected Fx lifecycle to stop after application Run")
					}
					appRan = true
					return nil
				}}
			},
			func() languageProvider {
				return stubLanguageProvider{language: i18n.LangEN}
			},
		),
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(context.Context) error {
					started = true
					return nil
				},
				OnStop: func(context.Context) error {
					stopped = true
					return nil
				},
			})
		}),
	)

	if !started {
		t.Error("expected Fx application to start")
	}
	if !appRan {
		t.Error("expected application Run to be called")
	}
	if !stopped {
		t.Error("expected Fx application to stop")
	}
}

func TestNewDBCreatesDatabaseAndRunsMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "filatrack.db")

	db, err := newDB(&config.Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})

	var tableName string
	err = db.Conn().QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'app_settings'",
	).Scan(&tableName)
	if err != nil {
		t.Fatalf("checking migrated schema: %v", err)
	}
	if tableName != "app_settings" {
		t.Fatalf("expected app_settings table, got %q", tableName)
	}
}

func TestNewDBReturnsOpenError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "filatrack.db")

	db, err := newDB(&config.Config{DBPath: dbPath})
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected database opening to fail")
	}
	if db != nil {
		_ = db.Close()
		t.Fatal("expected nil database after opening failure")
	}
}

func TestNewDBReturnsMigrationError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conflicting.db")
	preparedDB, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("preparing database: %v", err)
	}
	if _, err := preparedDB.Conn().Exec("CREATE TABLE spools (id TEXT PRIMARY KEY)"); err != nil {
		_ = preparedDB.Close()
		t.Fatalf("creating conflicting schema: %v", err)
	}
	if err := preparedDB.Close(); err != nil {
		t.Fatalf("closing prepared database: %v", err)
	}

	db, err := newDB(&config.Config{DBPath: dbPath})
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected migration to fail")
	}
	if db != nil {
		_ = db.Close()
		t.Fatal("expected nil database after migration failure")
	}
	if !strings.Contains(err.Error(), "running migrations") {
		t.Fatalf("expected migration error, got %v", err)
	}
}

func TestRunAppSetsLanguageBeforeRunning(t *testing.T) {
	originalLanguage := i18n.GetLanguage()
	originalExit := exitProcess
	t.Cleanup(func() {
		i18n.SetLanguage(originalLanguage)
		exitProcess = originalExit
	})

	exitProcess = func(code int) {
		t.Fatalf("unexpected process exit with code %d", code)
	}
	called := false
	app := stubApplication{run: func() error {
		called = true
		if got := i18n.GetLanguage(); got != i18n.LangBG {
			t.Errorf("expected language %q before Run, got %q", i18n.LangBG, got)
		}
		return nil
	}}

	runApp(app, stubLanguageProvider{language: i18n.LangBG})

	if !called {
		t.Fatal("expected application Run to be called")
	}
}

func TestRunAppLogsErrorAndExits(t *testing.T) {
	originalLanguage := i18n.GetLanguage()
	originalExit := exitProcess
	originalLogger := slog.Default()
	t.Cleanup(func() {
		i18n.SetLanguage(originalLanguage)
		exitProcess = originalExit
		slog.SetDefault(originalLogger)
	})

	var logOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))

	exitCode := 0
	exitProcess = func(code int) {
		exitCode = code
	}
	runError := errors.New("GTK failed")
	app := stubApplication{run: func() error {
		return runError
	}}

	runApp(app, stubLanguageProvider{language: i18n.LangEN})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if output := logOutput.String(); !strings.Contains(output, "application error") || !strings.Contains(output, runError.Error()) {
		t.Fatalf("expected application error in log, got %q", output)
	}
}
