package sqlite

import (
	"database/sql"
	"fmt"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) Close() error {
	return db.conn.Close()
}
