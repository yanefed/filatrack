package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"awesomeProject/internal/config"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/repository/postgres"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/repository/transfer"
)

type backend struct {
	conn          *sql.DB
	close         func() error
	spools        repository.SpoolRepository
	prints        repository.PrintJobRepository
	dryings       repository.DryingSessionRepository
	settings      repository.SettingsRepository
	printers      repository.PrinterRepository
	models        repository.PrinterModelRepository
	previewImages repository.PreviewImageRepository
}

type Manager struct {
	config         *config.Config
	selection      config.DatabaseSelection
	active         *backend
	logger         *slog.Logger
	stopActivity   func() error
	resumeActivity func(context.Context) error
}

func (m *Manager) SetActivityControl(stop func() error, resume func(context.Context) error) {
	m.stopActivity, m.resumeActivity = stop, resume
}

func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Manager, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Dir(cfg.DBPath)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	selection, err := config.LoadDatabaseSelection(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("loading database selection: %w", err)
	}
	m := &Manager{config: cfg, selection: selection, logger: logger}
	m.active, err = m.open(ctx, selection)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) open(ctx context.Context, selection config.DatabaseSelection) (*backend, error) {
	switch selection.Driver {
	case config.DriverSQLite:
		path := selection.SQLitePath
		if path == "" {
			path = m.config.DBPath
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		db, err := sqlite.New(path)
		if err != nil {
			return nil, err
		}
		if err := db.Migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return &backend{
			conn: db.Conn(), close: db.Close, spools: sqlite.NewSpoolRepo(db), prints: sqlite.NewPrintJobRepo(db),
			dryings: sqlite.NewDryingRepoWithLogger(db, m.logger), settings: sqlite.NewSettingsRepo(db),
			printers: sqlite.NewPrinterRepo(db), models: sqlite.NewPrinterModelRepo(db),
			previewImages: sqlite.NewPreviewImageRepo(db),
		}, nil
	case config.DriverPostgres:
		db, err := postgres.New(ctx, selection.PostgresURL)
		if err != nil {
			return nil, err
		}
		if err := db.Migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return &backend{
			conn: db.Conn(), close: db.Close, spools: db.Spools(), prints: db.Prints(), dryings: db.Dryings(m.logger),
			settings: db.Settings(), printers: db.Printers(), models: db.PrinterModels(), previewImages: db.PreviewImages(),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported database %q", selection.Driver)
	}
}

func (m *Manager) Close() error                                { return m.active.close() }
func (m *Manager) Selection() config.DatabaseSelection         { return m.selection }
func (m *Manager) Spools() repository.SpoolRepository          { return m.active.spools }
func (m *Manager) Prints() repository.PrintJobRepository       { return m.active.prints }
func (m *Manager) Dryings() repository.DryingSessionRepository { return m.active.dryings }
func (m *Manager) Settings() repository.SettingsRepository     { return m.active.settings }
func (m *Manager) Printers() repository.PrinterRepository      { return m.active.printers }
func (m *Manager) Models() repository.PrinterModelRepository   { return m.active.models }
func (m *Manager) PreviewImages() repository.PreviewImageRepository {
	return m.active.previewImages
}

// Switch changes the database used on the next launch. The caller should end
// the current application session after success so no later writes hit source.
func (m *Manager) Switch(ctx context.Context, next config.DatabaseSelection, migrate bool) (result error) {
	if err := next.Validate(); err != nil {
		return err
	}
	if next.SQLitePath != "" {
		absolute, err := filepath.Abs(next.SQLitePath)
		if err != nil {
			return fmt.Errorf("resolving SQLite path: %w", err)
		}
		next.SQLitePath = absolute
	}
	if next == m.selection {
		return fmt.Errorf("database selection is unchanged")
	}
	if next.Driver == m.selection.Driver && next.Driver == config.DriverSQLite {
		currentPath, nextPath := m.selection.SQLitePath, next.SQLitePath
		if currentPath == "" {
			currentPath = m.config.DBPath
		}
		if nextPath == "" {
			nextPath = m.config.DBPath
		}
		currentPath, _ = filepath.Abs(currentPath)
		nextPath, _ = filepath.Abs(nextPath)
		if currentPath == nextPath {
			return fmt.Errorf("SQLite database is already selected")
		}
	}
	if next.Driver == config.DriverPostgres && m.selection.Driver == config.DriverPostgres && next.PostgresURL == m.selection.PostgresURL {
		return fmt.Errorf("database selection is unchanged")
	}
	if m.stopActivity != nil {
		if err := m.stopActivity(); err != nil {
			return fmt.Errorf("stopping printer activity: %w", err)
		}
		defer func() {
			if result != nil && m.resumeActivity != nil {
				if err := m.resumeActivity(context.Background()); err != nil {
					result = fmt.Errorf("%w; resuming printer activity: %v", result, err)
				}
			}
		}()
	}
	target, err := m.open(ctx, next)
	if err != nil {
		return fmt.Errorf("opening target database: %w", err)
	}
	defer target.close()
	if migrate {
		if err := transfer.Copy(ctx, m.active.conn, target.conn, next.Driver); err != nil {
			return err
		}
	}
	if err := config.SaveDatabaseSelection(m.config.DataDir, next); err != nil {
		return fmt.Errorf("saving database selection: %w", err)
	}
	return nil
}
