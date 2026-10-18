package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yanefed/filatrack/internal/config"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/repository/transfer"
)

var ErrReadOnly = errors.New("offline cache is read-only; reconnect to PostgreSQL to make changes")

func (m *Manager) cachePath() string {
	// Separate caches prevent displaying data belonging to another server.
	// Hashing the full URL also avoids storing connection credentials in names.
	key := sha256.Sum256([]byte(m.selection.PostgresURL))
	return filepath.Join(m.config.DataDir, "cache", "database", fmt.Sprintf("%x.db", key))
}

func (m *Manager) sqliteBackend(db *sqlite.DB) *backend {
	return &backend{
		conn: db.Conn(), close: db.Close, spools: sqlite.NewSpoolRepo(db), prints: sqlite.NewPrintJobRepo(db),
		dryings: sqlite.NewDryingRepoWithLogger(db, m.logger), settings: sqlite.NewSettingsRepo(db),
		printers: sqlite.NewPrinterRepo(db), models: sqlite.NewPrinterModelRepo(db),
		previewImages: sqlite.NewPreviewImageRepo(db),
	}
}

func (m *Manager) openOffline(ctx context.Context) (err error) {
	db, err := sqlite.OpenReadOnly(ctx, m.cachePath())
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing offline snapshot: %w", closeErr))
			}
		}
	}()
	var stamp string
	err = db.Conn().QueryRowContext(ctx, "SELECT updated_at FROM snapshot_state WHERE id=1").Scan(&stamp)
	if err == nil {
		m.cacheUpdatedAt, err = time.Parse(time.RFC3339, stamp)
	}
	if err != nil {
		return fmt.Errorf("no completed snapshot available: %w", err)
	}
	// On success, the manager owns db and closes it on reconnect or shutdown.
	m.active = m.sqliteBackend(db)
	m.readOnly = true
	return nil
}

func (m *Manager) initializeCache() (err error) {
	path := m.cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	db, err := sqlite.New(path)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing local database cache: %w", closeErr))
			}
		}
	}()
	if err := db.Migrate(); err != nil {
		return err
	}
	// On success, the manager owns this writable cache connection.
	m.cache = db
	return nil
}

// refreshCache runs synchronously after a successful primary mutation. Its own
// bounded context allows caching a committed write even if its caller cancelled.
// The transaction preserves the previous snapshot on any failure.
func (m *Manager) refreshCache() {
	if m.cache == nil {
		if err := m.initializeCache(); err != nil {
			m.logger.Warn("initializing local database cache failed", "error", err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := transfer.ReplaceSQLite(ctx, m.active.conn, m.cache.Conn()); err != nil {
		m.logger.Warn("refreshing local database cache failed; previous snapshot retained", "error", err)
	}
}

func (m *Manager) write(run func() error) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	if m.readOnly {
		return ErrReadOnly
	}
	if err := run(); err != nil {
		return err
	}
	m.refreshCache()
	return nil
}

// Reconnect replaces the offline snapshot with PostgreSQL. Existing services
// keep their repository handles; those handles route to the current backend.
func (m *Manager) Reconnect(ctx context.Context) error {
	return m.reconnect(ctx, m.open)
}

func (m *Manager) reconnect(ctx context.Context, open func(context.Context, config.DatabaseSelection) (*backend, error)) error {
	m.writeMu.Lock()
	m.stateMu.RLock()
	readOnly, closed := m.readOnly, m.closed
	m.stateMu.RUnlock()
	if closed {
		m.writeMu.Unlock()
		return fmt.Errorf("database manager is closed")
	}
	if !readOnly {
		m.writeMu.Unlock()
		return nil
	}
	primary, err := open(ctx, m.selection)
	if err != nil {
		m.writeMu.Unlock()
		return fmt.Errorf("reconnecting to PostgreSQL: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = primary.close()
		m.writeMu.Unlock()
		return err
	}
	m.stateMu.Lock()
	previous := m.active
	m.active = primary
	m.readOnly = false
	// Acquiring the exclusive lock waited for all reads of the old snapshot.
	if err := previous.close(); err != nil {
		m.logger.Warn("closing offline snapshot", "error", err)
	}
	m.stateMu.Unlock()
	m.refreshCache()
	m.writeMu.Unlock()
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	m.stateMu.RLock()
	closed = m.closed
	m.stateMu.RUnlock()
	if closed {
		return nil
	}
	if m.resumeActivity != nil {
		// The backend has already switched. Cache refresh may have outlived
		// the retry deadline, so give monitor startup its own bounded context.
		resumeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.resumeActivity(resumeCtx); err != nil {
			// The database is online even when starting printer activity fails.
			m.logger.Warn("resuming printer monitoring after reconnect", "error", err)
		}
	}
	return nil
}

func readRepository[T any](m *Manager, run func(*backend) (T, error)) (T, error) {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return run(m.active)
}
