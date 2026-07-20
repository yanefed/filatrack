package moonraker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"awesomeProject/internal/domain"
)

type PrinterLister interface {
	List(ctx context.Context) ([]domain.Printer, error)
}

type Runner struct {
	Printers     PrinterLister
	Sink         Sink
	Dialer       Dialer
	Metadata     MetadataFetcher
	Logger       *slog.Logger
	SyncInterval time.Duration

	mu       sync.Mutex
	runCtx   context.Context
	cancel   context.CancelFunc
	monitors map[string]*monitorHandle
	wg       sync.WaitGroup
}

type monitorHandle struct {
	cancel context.CancelFunc
}

func NewRunner(printers PrinterLister, sink Sink, dialer Dialer) *Runner {
	return &Runner{
		Printers: printers,
		Sink:     sink,
		Dialer:   dialer,
		Logger:   slog.Default(),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return nil
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
	if r.SyncInterval <= 0 {
		r.SyncInterval = 5 * time.Second
	}
	runCtx, cancel := context.WithCancel(context.Background())
	r.runCtx = runCtx
	r.cancel = cancel
	r.monitors = make(map[string]*monitorHandle)
	r.mu.Unlock()

	if err := r.Sync(ctx); err != nil {
		_ = r.Stop(ctx)
		return err
	}

	r.wg.Add(1)
	go r.syncLoop(runCtx)
	return nil
}

func (r *Runner) Sync(ctx context.Context) error {
	printers, err := r.Printers.List(ctx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runCtx == nil {
		return nil
	}

	wanted := make(map[string]domain.Printer)
	for _, printer := range printers {
		if !printer.Enabled || printer.Kind != domain.PrinterKindMoonraker {
			continue
		}
		wanted[printer.ID] = printer
		if _, running := r.monitors[printer.ID]; !running {
			r.startMonitorLocked(printer)
		}
	}

	for printerID, monitor := range r.monitors {
		if _, ok := wanted[printerID]; !ok {
			monitor.cancel()
			delete(r.monitors, printerID)
		}
	}
	return nil
}

func (r *Runner) Stop(context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	monitorCancels := make([]context.CancelFunc, 0, len(r.monitors))
	for _, monitor := range r.monitors {
		monitorCancels = append(monitorCancels, monitor.cancel)
	}
	r.cancel = nil
	r.runCtx = nil
	r.monitors = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, monitorCancel := range monitorCancels {
		monitorCancel()
	}
	r.wg.Wait()
	return nil
}

func (r *Runner) syncLoop(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Sync(ctx); err != nil && ctx.Err() == nil {
				r.Logger.Error("moonraker printer sync failed", "error", err)
			}
		}
	}
}

func (r *Runner) startMonitorLocked(printer domain.Printer) {
	monitorCtx, cancel := context.WithCancel(r.runCtx)
	handle := &monitorHandle{cancel: cancel}
	r.monitors[printer.ID] = handle
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		monitor := Monitor{
			Printer:  printer,
			Dialer:   r.Dialer,
			Sink:     r.Sink,
			Metadata: r.Metadata,
		}
		if err := monitor.Run(monitorCtx); err != nil && monitorCtx.Err() == nil {
			r.Logger.Error("moonraker monitor stopped", "printer_id", printer.ID, "printer", printer.Name, "error", err)
		}

		r.mu.Lock()
		if r.monitors != nil && r.monitors[printer.ID] == handle {
			delete(r.monitors, printer.ID)
		}
		r.mu.Unlock()
	}()
}
