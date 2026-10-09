package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/moonraker"
	"github.com/yanefed/filatrack/internal/port"
)

type historyReplayConn struct{ messages []json.RawMessage }

func (c *historyReplayConn) WriteJSON(any) error { return nil }
func (c *historyReplayConn) Close() error        { return nil }
func (c *historyReplayConn) ReadJSON(target any) error {
	if len(c.messages) == 0 {
		return errors.New("replay finished")
	}
	*target.(*json.RawMessage) = c.messages[0]
	c.messages = c.messages[1:]
	return nil
}

type historyReplayDialer struct{ conn *historyReplayConn }

func (d historyReplayDialer) Dial(context.Context, domain.Printer) (moonraker.Conn, error) {
	return d.conn, nil
}

func TestMultipleClientsRecordOnePrint(t *testing.T) {
	for _, state := range []string{"completed", "cancelled", "error"} {
		t.Run(state, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shared.db")
			apps := make([]applicationServices, 3)
			for i := range apps {
				apps[i] = openApplication(t, path)
				db := apps[i].db
				t.Cleanup(func() {
					if err := db.Close(); err != nil {
						t.Errorf("closing client DB: %v", err)
					}
				})
			}
			spool := &domain.Spool{Manufacturer: "Test", Material: domain.MaterialPLA, Color: "Black", ColorHex: "#000000", TotalWeightG: 1000}
			if err := apps[0].spools.Create(t.Context(), spool); err != nil {
				t.Fatal(err)
			}
			printer := domain.Printer{Name: "Test", MoonrakerURL: "http://printer"}
			if err := apps[0].printers.CreateMoonrakerPrinter(t.Context(), &printer); err != nil {
				t.Fatal(err)
			}
			if err := apps[0].printers.AssignSpool(t.Context(), printer.ID, spool.ID); err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			failures := make(chan error, len(apps))
			var wg sync.WaitGroup
			for i, app := range apps {
				wg.Go(func() {
					conn := &historyReplayConn{messages: []json.RawMessage{
						json.RawMessage(`{"method":"notify_status_update","params":[{"print_stats":{"state":"printing","filename":"part.gcode"},"virtual_sdcard":{"progress":0.42}}]}`),
						json.RawMessage(fmt.Sprintf(`{"method":"notify_history_changed","params":[{"action":"finished","job":{"job_id":"000042","filename":"part.gcode","status":%q,"end_time":1060,"filament_used":1000,"print_duration":60}}]}`, state)),
						// Replay the same notification after reconnect/delivery duplication.
						json.RawMessage(fmt.Sprintf(`{"method":"notify_history_changed","params":[{"action":"finished","job":{"job_id":"000042","filename":"part.gcode","status":%q,"end_time":1060,"filament_used":1000,"print_duration":60}}]}`, state)),
					}}
					monitor := moonraker.Monitor{Printer: printer, Dialer: historyReplayDialer{conn: conn}, Sink: app.printers,
						Now: func() time.Time { return time.Unix(1000+int64(i*100), 0) }}
					<-start
					err := monitor.Run(t.Context())
					if err == nil || err.Error() != "reading moonraker websocket: replay finished" {
						failures <- err
					}
				})
			}
			close(start)
			wg.Wait()
			close(failures)
			for err := range failures {
				t.Errorf("monitor failed: %v", err)
			}
			jobs, err := apps[0].prints.List(t.Context(), domain.PrintJobFilter{})
			if err != nil || len(jobs) != 1 {
				t.Fatalf("jobs=%+v error=%v", jobs, err)
			}
			job := jobs[0]
			expected := domain.WeightFromLength(1000, spool.DiameterMM, spool.DensityGCM3)
			if job.ExternalPrintID != "000042" || job.SpoolID != spool.ID || job.PrintedAt.Unix() != 1060 || math.Abs(job.ActualWeightG-expected) > 1e-8 {
				t.Fatalf("stored job=%+v expected weight=%v", job, expected)
			}
			_, remaining, err := apps[0].spools.GetWithRemaining(t.Context(), spool.ID)
			used := spool.TotalWeightG - remaining
			if err != nil || math.Abs(used-expected) > 1e-8 {
				t.Fatalf("used=%v expected=%v error=%v", used, expected, err)
			}
		})
	}
}

// The embedded port supplies unused operations; this client serves only history
// and blocks metadata until both clients have checked the empty local history.
type racingHistoryClient struct {
	port.PrinterJobClient
	arrived chan<- struct{}
	release <-chan struct{}
}

func (c racingHistoryClient) ListHistory(context.Context, domain.Printer, int) ([]domain.PrinterHistoryJob, error) {
	return []domain.PrinterHistoryJob{{ExternalPrintID: "000042", GcodeFilename: "part.gcode", EndedAt: time.Unix(1060, 0), Status: domain.PrinterHistoryStatusCompleted}}, nil
}
func (c racingHistoryClient) FetchMetadata(ctx context.Context, _ domain.Printer, _ string) (*domain.GcodeMetadata, error) {
	c.arrived <- struct{}{}
	select {
	case <-c.release:
		return &domain.GcodeMetadata{FilamentWeightG: 10}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestConcurrentHistoryImportsCountOnlyInsertedJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	apps := []applicationServices{openApplication(t, path), openApplication(t, path)}
	for _, app := range apps {
		db := app.db
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Errorf("closing client DB: %v", err)
			}
		})
	}
	printer := domain.Printer{Name: "Test", MoonrakerURL: "http://printer"}
	if err := apps[0].printers.CreateMoonrakerPrinter(t.Context(), &printer); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	arrived, release := make(chan struct{}, 2), make(chan struct{})
	type result struct {
		n   int
		err error
	}
	results := make(chan result, 2)
	for _, app := range apps {
		app.printers.SetJobClient(racingHistoryClient{arrived: arrived, release: release})
		go func() { n, err := app.printers.SyncPrinterHistory(ctx, printer.ID, 10); results <- result{n, err} }()
	}
	for range 2 {
		select {
		case <-arrived:
		case <-ctx.Done():
			t.Fatal("clients did not reach concurrent insert")
		}
	}
	close(release)
	total := 0
	for range 2 {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		total += r.n
	}
	jobs, err := apps[0].prints.List(t.Context(), domain.PrintJobFilter{})
	if total != 1 || err != nil || len(jobs) != 1 {
		t.Fatalf("created=%d jobs=%+v error=%v", total, jobs, err)
	}
}
