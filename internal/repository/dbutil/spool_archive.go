package dbutil

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

// The spool row is locked before printer rows, also by AssignSpool. SQLite's
// initial write obtains its writer lock; PostgreSQL uses row locks.
func lockSpool(ctx context.Context, tx *sql.Tx, id string, pg bool) error {
	result, err := tx.ExecContext(ctx, bind("UPDATE spools SET archived = archived WHERE id = ?", pg), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("spool not found: %s", id)
	}
	return nil
}

func bind(query string, pg bool) string {
	if !pg {
		return query
	}
	n := 0
	var b strings.Builder
	for _, c := range query {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func ArchiveSpool(ctx context.Context, db *sql.DB, id string, detachPrinterIDs []string, pg bool) (domain.SpoolArchive, error) {
	receipt := domain.SpoolArchive{SpoolID: id, ArchivedAt: time.Now().UTC().Truncate(time.Microsecond)}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return receipt, err
	}
	defer tx.Rollback()
	if err := lockSpool(ctx, tx, id, pg); err != nil {
		return receipt, err
	}
	var archived int
	if err := tx.QueryRowContext(ctx, bind("SELECT archived FROM spools WHERE id = ?", pg), id).Scan(&archived); err != nil {
		return receipt, err
	}
	if archived != 0 {
		return receipt, domain.ErrArchiveChanged
	}
	query := "SELECT id, name, state FROM printers WHERE active_spool_id = ? ORDER BY id"
	if pg {
		query += " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, bind(query, pg), id)
	if err != nil {
		return receipt, err
	}
	for rows.Next() {
		var p domain.Printer
		if err := rows.Scan(&p.ID, &p.Name, &p.State); err != nil {
			rows.Close()
			return receipt, err
		}
		receipt.Printers = append(receipt.Printers, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return receipt, err
	}
	for _, p := range receipt.Printers {
		if p.State == domain.PrinterStatePrinting || p.State == domain.PrinterStatePaused {
			return receipt, domain.ErrSpoolInUse
		}
	}
	allowed := make(map[string]bool, len(detachPrinterIDs))
	for _, id := range detachPrinterIDs {
		allowed[id] = true
	}
	for _, printer := range receipt.Printers {
		if !allowed[printer.ID] {
			return receipt, domain.ErrSpoolAssigned
		}
	}
	if _, err := tx.ExecContext(ctx, bind("UPDATE printers SET active_spool_id = NULL, updated_at = ? WHERE active_spool_id = ?", pg), receipt.ArchivedAt, id); err != nil {
		return receipt, err
	}
	if _, err := tx.ExecContext(ctx, bind("UPDATE spools SET archived = 1, updated_at = ? WHERE id = ?", pg), receipt.ArchivedAt, id); err != nil {
		return receipt, err
	}
	return receipt, tx.Commit()
}

func RestoreSpool(ctx context.Context, db *sql.DB, receipt domain.SpoolArchive, pg bool) (domain.SpoolRestoreResult, error) {
	var result domain.SpoolRestoreResult
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err := lockSpool(ctx, tx, receipt.SpoolID, pg); err != nil {
		return result, err
	}
	query := "UPDATE spools SET archived = 0, updated_at = ? WHERE id = ? AND archived = 1"
	args := []any{time.Now().UTC(), receipt.SpoolID}
	if !receipt.ArchivedAt.IsZero() {
		query += " AND updated_at = ?"
		args = append(args, receipt.ArchivedAt)
	}
	changed, err := tx.ExecContext(ctx, bind(query, pg), args...)
	if err != nil {
		return result, err
	}
	n, err := changed.RowsAffected()
	if err != nil {
		return result, err
	}
	if n == 0 {
		return result, domain.ErrArchiveChanged
	}
	for _, printer := range receipt.Printers {
		changed, err := tx.ExecContext(ctx, bind("UPDATE printers SET active_spool_id = ?, updated_at = ? WHERE id = ? AND (active_spool_id IS NULL OR active_spool_id = '') AND state = 'ready'", pg), receipt.SpoolID, time.Now().UTC(), printer.ID)
		if err != nil {
			return result, err
		}
		n, err := changed.RowsAffected()
		if err != nil {
			return result, err
		}
		if n == 0 {
			result.SkippedPrinterIDs = append(result.SkippedPrinterIDs, printer.ID)
		}
	}
	return result, tx.Commit()
}

func AssignSpool(ctx context.Context, db *sql.DB, printerID, spoolID string, pg bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if spoolID != "" {
		if err := lockSpool(ctx, tx, spoolID, pg); err != nil {
			return err
		}
		var archived int
		if err := tx.QueryRowContext(ctx, bind("SELECT archived FROM spools WHERE id = ?", pg), spoolID).Scan(&archived); err != nil {
			return err
		}
		if archived != 0 {
			return domain.ErrArchiveChanged
		}
	}
	var assigned any
	if spoolID != "" {
		assigned = spoolID
	}
	result, err := tx.ExecContext(ctx, bind("UPDATE printers SET active_spool_id = ?, updated_at = ? WHERE id = ?", pg), assigned, time.Now().UTC(), printerID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("printer not found: %s", printerID)
	}
	return tx.Commit()
}
