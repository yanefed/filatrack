package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/port"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yanefed/filatrack/internal/config"
	"github.com/yanefed/filatrack/internal/repository/postgres"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/repository/transfer"
)

type backend struct {
	conn          *sql.DB
	close         func() error
	spools        port.SpoolRepository
	prints        port.PrintJobRepository
	dryings       port.DryingSessionRepository
	settings      port.SettingsRepository
	printers      port.PrinterRepository
	models        port.PrinterModelRepository
	previewImages port.PreviewImageRepository
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
	if logger == nil {
		logger = slog.Default()
	}
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
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing SQLite database after migration failure: %w", closeErr))
			}
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
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing PostgreSQL database after migration failure: %w", closeErr))
			}
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

func (m *Manager) Close() error                          { return m.active.close() }
func (m *Manager) Selection() config.DatabaseSelection   { return m.selection }
func (m *Manager) Spools() port.SpoolRepository          { return m.active.spools }
func (m *Manager) Prints() port.PrintJobRepository       { return m.active.prints }
func (m *Manager) Dryings() port.DryingSessionRepository { return m.active.dryings }
func (m *Manager) Settings() port.SettingsRepository     { return m.active.settings }
func (m *Manager) Printers() port.PrinterRepository      { return m.active.printers }
func (m *Manager) Models() port.PrinterModelRepository   { return m.active.models }
func (m *Manager) PreviewImages() port.PreviewImageRepository {
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
		var err error
		currentPath, err = filepath.Abs(currentPath)
		if err != nil {
			return fmt.Errorf("resolving current SQLite path: %w", err)
		}
		nextPath, err = filepath.Abs(nextPath)
		if err != nil {
			return fmt.Errorf("resolving target SQLite path: %w", err)
		}
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
					result = errors.Join(result, fmt.Errorf("resuming printer activity: %w", err))
				}
			}
		}()
	}
	target, err := m.open(ctx, next)
	if err != nil {
		return fmt.Errorf("opening target database: %w", err)
	}
	defer func() {
		if err := target.close(); err != nil {
			if result != nil {
				result = errors.Join(result, fmt.Errorf("closing target database: %w", err))
				return
			}
			// A successful switch has already saved the next-launch selection.
			// A cleanup failure must not resume writes to the old database.
			m.logger.Error("closing target database failed", "driver", next.Driver, "error", err)
		}
	}()
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
