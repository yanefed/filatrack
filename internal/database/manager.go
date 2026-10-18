package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yanefed/filatrack/internal/port"

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
	repositories   *backend
	logger         *slog.Logger
	stopActivity   func() error
	resumeActivity func(context.Context) error
	writeMu        sync.Mutex
	stateMu        sync.RWMutex
	activityMu     sync.Mutex
	cache          *sqlite.DB
	readOnly       bool
	closed         bool
	cacheUpdatedAt time.Time
}

func (m *Manager) SetActivityControl(stop func() error, resume func(context.Context) error) {
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
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
		if selection.Driver != config.DriverPostgres {
			return nil, err
		}
		if cacheErr := m.openOffline(ctx); cacheErr != nil {
			return nil, errors.Join(err, fmt.Errorf("opening offline cache: %w", cacheErr))
		}
		logger.Warn("PostgreSQL unavailable; using read-only local cache", "error", err, "cached_at", m.cacheUpdatedAt)
	} else if selection.Driver == config.DriverPostgres {
		// Cache failures must not turn a committed primary write into an error.
		// A later successful write will retry initialization and refresh.
		m.refreshCache()
	}
	if selection.Driver == config.DriverPostgres {
		m.wrapRepositories()
	}
	return m, nil
}

func (m *Manager) open(ctx context.Context, selection config.DatabaseSelection) (result *backend, err error) {
	switch selection.Driver {
	case config.DriverSQLite:
		path := selection.SQLitePath
		if path == "" {
			path = m.config.DBPath
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		var db *sqlite.DB
		db, err = sqlite.New(path)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				if closeErr := db.Close(); closeErr != nil {
					err = errors.Join(err, fmt.Errorf("closing SQLite database: %w", closeErr))
				}
			}
		}()
		if err := db.Migrate(); err != nil {
			return nil, err
		}
		// The returned backend owns db and exposes its Close method.
		return m.sqliteBackend(db), nil
	case config.DriverPostgres:
		var db *postgres.DB
		db, err = postgres.New(ctx, selection.PostgresURL)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				if closeErr := db.Close(); closeErr != nil {
					err = errors.Join(err, fmt.Errorf("closing PostgreSQL database: %w", closeErr))
				}
			}
		}()
		if err := db.MigrateContext(ctx); err != nil {
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

func (m *Manager) Close() error {
	// Serialize shutdown with activity resumed by an asynchronous reconnect.
	// Stop workers before taking writeMu so their pending writes can finish.
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	var stopErr error
	if m.stopActivity != nil {
		stopErr = m.stopActivity()
	}
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	if m.closed {
		return stopErr
	}
	m.closed = true
	err := errors.Join(stopErr, m.active.close())
	if m.cache != nil {
		err = errors.Join(err, m.cache.Close())
	}
	return err
}

func (m *Manager) ReadOnly() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.readOnly
}
func (m *Manager) CacheUpdatedAt() time.Time {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.cacheUpdatedAt
}

func (m *Manager) repositoryBackend() *backend {
	if m.repositories != nil {
		return m.repositories
	}
	return m.active
}

func (m *Manager) Selection() config.DatabaseSelection   { return m.selection }
func (m *Manager) Spools() port.SpoolRepository          { return m.repositoryBackend().spools }
func (m *Manager) Prints() port.PrintJobRepository       { return m.repositoryBackend().prints }
func (m *Manager) Dryings() port.DryingSessionRepository { return m.repositoryBackend().dryings }
func (m *Manager) Settings() port.SettingsRepository     { return m.repositoryBackend().settings }
func (m *Manager) Printers() port.PrinterRepository      { return m.repositoryBackend().printers }
func (m *Manager) Models() port.PrinterModelRepository   { return m.repositoryBackend().models }
func (m *Manager) PreviewImages() port.PreviewImageRepository {
	return m.repositoryBackend().previewImages
}

// Switch changes the database used on the next launch. The caller should end
// the current application session after success so no later writes hit source.
func (m *Manager) Switch(ctx context.Context, next config.DatabaseSelection, migrate bool) (result error) {
	if m.ReadOnly() {
		return ErrReadOnly
	}
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
