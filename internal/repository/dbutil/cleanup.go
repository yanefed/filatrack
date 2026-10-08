// Package dbutil handles cleanup of SQL resources without losing primary errors.
package dbutil

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
)

func Close(resource io.Closer, result *error) {
	if err := resource.Close(); err != nil {
		*result = errors.Join(*result, fmt.Errorf("closing database resource: %w", err))
	}
}

func Rollback(tx interface{ Rollback() error }, result *error) {
	// Commit and automatic rollback both make a deferred rollback redundant.
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		*result = errors.Join(*result, fmt.Errorf("rolling back transaction: %w", err))
	}
}
