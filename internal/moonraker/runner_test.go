package moonraker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"awesomeProject/internal/domain"
)

type fakePrinterLister struct {
	printers []domain.Printer
	err      error
}

func (l *fakePrinterLister) List(context.Context) ([]domain.Printer, error) {
	return l.printers, l.err
}

type blockingConn struct {
	once sync.Once
	done chan struct{}
}

func (c *blockingConn) WriteJSON(any) error { return nil }
func (c *blockingConn) ReadJSON(any) error {
	<-c.done
	return errors.New("closed")
}
func (c *blockingConn) Close() error {
	c.once.Do(func() { close(c.done) })
	return nil
}

type countingDialer struct {
	count  int
	conn   Conn
	dialed chan struct{}
}

func (d *countingDialer) Dial(context.Context, domain.Printer) (Conn, error) {
	d.count++
	if d.dialed != nil {
		d.dialed <- struct{}{}
	}
	return d.conn, nil
}

func TestRunnerStartsEnabledMoonrakerPrinters(t *testing.T) {
	done := make(chan struct{})
	dialed := make(chan struct{}, 1)
	dialer := &countingDialer{conn: &blockingConn{done: done}, dialed: dialed}
	runner := NewRunner(
		&fakePrinterLister{printers: []domain.Printer{
			{ID: "enabled", Name: "Enabled", Kind: domain.PrinterKindMoonraker, Enabled: true},
			{ID: "disabled", Name: "Disabled", Kind: domain.PrinterKindMoonraker, Enabled: false},
			{ID: "unknown-kind", Name: "Unknown", Kind: "other", Enabled: true},
		}},
		&fakeSink{},
		dialer,
	)
	runner.Logger = slog.New(slog.DiscardHandler)

	if err := runner.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-dialed
	if dialer.count != 1 {
		t.Fatalf("dial count = %d, want 1", dialer.count)
	}
	if err := runner.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRunnerStartReturnsListError(t *testing.T) {
	wantErr := errors.New("list failed")
	runner := NewRunner(&fakePrinterLister{err: wantErr}, &fakeSink{}, &countingDialer{})
	if err := runner.Start(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("Start error = %v", err)
	}
}

func TestRunnerSyncStartsNewPrinters(t *testing.T) {
	done := make(chan struct{})
	dialed := make(chan struct{}, 2)
	lister := &fakePrinterLister{printers: []domain.Printer{
		{ID: "first", Name: "First", Kind: domain.PrinterKindMoonraker, Enabled: true},
	}}
	dialer := &countingDialer{conn: &blockingConn{done: done}, dialed: dialed}
	runner := NewRunner(lister, &fakeSink{}, dialer)
	runner.Logger = slog.New(slog.DiscardHandler)

	if err := runner.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-dialed

	lister.printers = append(lister.printers, domain.Printer{ID: "second", Name: "Second", Kind: domain.PrinterKindMoonraker, Enabled: true})
	if err := runner.Sync(t.Context()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	<-dialed
	if dialer.count != 2 {
		t.Fatalf("dial count = %d, want 2", dialer.count)
	}
	if err := runner.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRunnerSyncStopsDisabledPrinters(t *testing.T) {
	done := make(chan struct{})
	dialed := make(chan struct{}, 1)
	lister := &fakePrinterLister{printers: []domain.Printer{
		{ID: "printer", Name: "Printer", Kind: domain.PrinterKindMoonraker, Enabled: true},
	}}
	dialer := &countingDialer{conn: &blockingConn{done: done}, dialed: dialed}
	runner := NewRunner(lister, &fakeSink{}, dialer)
	runner.Logger = slog.New(slog.DiscardHandler)

	if err := runner.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-dialed

	lister.printers[0].Enabled = false
	if err := runner.Sync(t.Context()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := runner.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
