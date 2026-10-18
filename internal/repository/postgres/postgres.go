package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/port"
	"log/slog"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// DB provides PostgreSQL-backed implementations of all repository interfaces.
// PostgreSQL-specific repository implementations use PostgreSQL placeholders and transactions.
type DB struct {
	conn *sql.DB
}

func New(ctx context.Context, url string) (*DB, error) {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("opening PostgreSQL: %w", err)
	}
	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(2)
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(check); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing PostgreSQL connection: %w", closeErr))
		}
		return nil, fmt.Errorf("connecting to PostgreSQL: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (db *DB) Conn() *sql.DB { return db.conn }
func (db *DB) Close() error  { return db.conn.Close() }

//go:embed migrations/*.sql
var migrations embed.FS

func (db *DB) Migrate() error {
	return db.MigrateContext(context.Background())
}

func (db *DB) MigrateContext(ctx context.Context) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db.conn, "migrations"); err != nil {
		return fmt.Errorf("running PostgreSQL migrations: %w", err)
	}
	return nil
}

func (db *DB) Spools() port.SpoolRepository    { return NewSpoolRepo(db) }
func (db *DB) Prints() port.PrintJobRepository { return NewPrintJobRepo(db) }
func (db *DB) Dryings(logger *slog.Logger) port.DryingSessionRepository {
	return NewDryingRepoWithLogger(db, logger)
}
func (db *DB) Settings() port.SettingsRepository { return NewSettingsRepo(db) }
func (db *DB) Printers() port.PrinterRepository  { return NewPrinterRepo(db) }
func (db *DB) PrinterModels() port.PrinterModelRepository {
	return NewPrinterModelRepo(db)
}
func (db *DB) PreviewImages() port.PreviewImageRepository {
	return NewPreviewImageRepo(db)
}

// SQL helpers keep PostgreSQL's placeholder and legacy INTEGER flag conventions
// inside this backend rather than leaking them into the SQLite implementation.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.conn.ExecContext(ctx, bindPostgres(query), postgresArgs(args)...)
}
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, bindPostgres(query), postgresArgs(args)...)
}
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.conn.QueryRowContext(ctx, bindPostgres(query), postgresArgs(args)...)
}
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx}, nil
}

type Tx struct{ *sql.Tx }

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.Tx.ExecContext(ctx, bindPostgres(query), postgresArgs(args)...)
}
func (tx *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return tx.Tx.PrepareContext(ctx, bindPostgres(query))
}
func postgresArgs(args []any) []any {
	converted := make([]any, len(args))
	for i, arg := range args {
		if flag, ok := arg.(bool); ok {
			if flag {
				converted[i] = 1
			} else {
				converted[i] = 0
			}
		} else {
			converted[i] = arg
		}
	}
	return converted
}
func bindPostgres(query string) string {
	var b strings.Builder
	n := 0
	for _, c := range query {
		if c == '?' {
			n++
			// strings.Builder writes cannot fail.
			_, _ = fmt.Fprintf(&b, "$%d", n)
		} else {
			_, _ = b.WriteRune(c)
		}
	}
	return b.String()
}
