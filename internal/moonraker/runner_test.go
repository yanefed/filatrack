package moonraker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

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
	err    error
	dialed chan struct{}
}

func (d *countingDialer) Dial(context.Context, domain.Printer) (Conn, error) {
	d.count++
	if d.dialed != nil {
		d.dialed <- struct{}{}
	}
	if d.err != nil {
		return nil, d.err
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
	if err := runner.Stop(); err != nil {
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
	if err := runner.Stop(); err != nil {
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
	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRunnerMarksPrinterOfflineWhenMonitorFails(t *testing.T) {
	dialed := make(chan struct{}, 1)
	sink := &fakeSink{}
	statusUpdated := make(chan struct{}, 1)
	runner := NewRunner(
		&fakePrinterLister{printers: []domain.Printer{
			{
				ID:          "printer",
				Name:        "Printer",
				Kind:        domain.PrinterKindMoonraker,
				Enabled:     true,
				State:       domain.PrinterStatePrinting,
				CurrentFile: "part.gcode",
				Progress:    0.5,
			},
		}},
		sink,
		&countingDialer{err: errors.New("dial failed"), dialed: dialed},
	)
	runner.Logger = slog.New(slog.DiscardHandler)
	runner.Sink = runnerSinkFunc(func(ctx context.Context, printer *domain.Printer) error {
		err := sink.UpdateStatus(ctx, printer)
		statusUpdated <- struct{}{}
		return err
	})

	if err := runner.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-dialed

	select {
	case <-statusUpdated:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for offline status")
	}

	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(sink.statuses) != 1 {
		t.Fatalf("statuses = %+v", sink.statuses)
	}
	status := sink.statuses[0]
	if status.State != domain.PrinterStateOffline || status.CurrentFile != "" || status.Progress != 0 || status.LastSeenAt.IsZero() {
		t.Fatalf("offline status = %+v", status)
	}
}

type runnerSinkFunc func(context.Context, *domain.Printer) error

func (f runnerSinkFunc) UpdateStatus(ctx context.Context, printer *domain.Printer) error {
	return f(ctx, printer)
}

func (f runnerSinkFunc) RecordPrinterPrint(context.Context, domain.PrinterPrintEvent) (*domain.PrintJob, error) {
	return &domain.PrintJob{}, nil
}
