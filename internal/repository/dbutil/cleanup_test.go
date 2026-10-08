package dbutil

import (
	"database/sql"
	"errors"
	"testing"
)

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

type rollbackFunc func() error

func (f rollbackFunc) Rollback() error { return f() }

func TestClosePreservesPrimaryAndCleanupErrors(t *testing.T) {
	primary := errors.New("query failed")
	cleanup := errors.New("close failed")
	result := primary
	Close(closeFunc(func() error { return cleanup }), &result)
	if !errors.Is(result, primary) || !errors.Is(result, cleanup) {
		t.Fatalf("cleanup lost an error: %v", result)
	}
	result = nil
	Close(closeFunc(func() error { return cleanup }), &result)
	if !errors.Is(result, cleanup) {
		t.Fatalf("successful operation hid cleanup failure: %v", result)
	}
	result = primary
	Close(closeFunc(func() error { return nil }), &result)
	if result != primary {
		t.Fatalf("successful cleanup changed the primary error: %v", result)
	}
}

func TestRollbackDistinguishesCompletedTransactionAndFailure(t *testing.T) {
	primary := errors.New("insert failed")
	rollbackErr := errors.New("rollback failed")
	result := primary
	Rollback(rollbackFunc(func() error { return rollbackErr }), &result)
	if !errors.Is(result, primary) || !errors.Is(result, rollbackErr) {
		t.Fatalf("rollback lost an error: %v", result)
	}
	for _, cause := range []error{nil, sql.ErrTxDone} {
		result = primary
		Rollback(rollbackFunc(func() error { return cause }), &result)
		if result != primary {
			t.Fatalf("completed rollback changed the primary error: %v", result)
		}
		result = nil
		Rollback(rollbackFunc(func() error { return cause }), &result)
		if result != nil {
			t.Fatalf("successful transaction reported rollback failure: %v", result)
		}
	}
}
