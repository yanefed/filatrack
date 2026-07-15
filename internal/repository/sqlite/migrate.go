package sqlite

import (
	"embed"
	"fmt"

	goose "github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func (db *DB) Migrate() error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}
	if err := goose.Up(db.conn, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}
