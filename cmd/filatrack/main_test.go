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

type stubShutdowner struct {
	called bool
	err    error
}

func (s *stubShutdowner) Shutdown(...fx.ShutdownOption) error {
	s.called = true
	return s.err
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
	var started, stopped bool

	startApplication(
		fx.NopLogger,
		fx.Invoke(func(lifecycle fx.Lifecycle, shutdowner fx.Shutdowner) {
			lifecycle.Append(fx.Hook{
				OnStart: func(context.Context) error {
					started = true
					return shutdowner.Shutdown()
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
	shutdowner := &stubShutdowner{}

	runApp(app, stubLanguageProvider{language: i18n.LangBG}, shutdowner)

	if !called {
		t.Fatal("expected application Run to be called")
	}
	if !shutdowner.called {
		t.Fatal("expected Fx application shutdown after application Run returns")
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
	shutdowner := &stubShutdowner{}

	runApp(app, stubLanguageProvider{language: i18n.LangEN}, shutdowner)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if shutdowner.called {
		t.Fatal("expected no Fx shutdown after application Run error")
	}
	if output := logOutput.String(); !strings.Contains(output, "application error") || !strings.Contains(output, runError.Error()) {
		t.Fatalf("expected application error in log, got %q", output)
	}
}

func TestRunAppLogsShutdownError(t *testing.T) {
	originalLanguage := i18n.GetLanguage()
	originalExit := exitProcess
	originalLogger := slog.Default()
	t.Cleanup(func() {
		i18n.SetLanguage(originalLanguage)
		exitProcess = originalExit
		slog.SetDefault(originalLogger)
	})

	exitProcess = func(code int) {
		t.Fatalf("unexpected process exit with code %d", code)
	}

	var logOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))

	shutdownErr := errors.New("shutdown failed")
	app := stubApplication{run: func() error {
		return nil
	}}
	shutdowner := &stubShutdowner{err: shutdownErr}

	runApp(app, stubLanguageProvider{language: i18n.LangEN}, shutdowner)

	if !shutdowner.called {
		t.Fatal("expected Fx application shutdown after application Run returns")
	}
	if output := logOutput.String(); !strings.Contains(output, "shutdown error") || !strings.Contains(output, shutdownErr.Error()) {
		t.Fatalf("expected shutdown error in log, got %q", output)
	}
}
