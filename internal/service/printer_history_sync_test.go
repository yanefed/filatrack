package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"testing"
	"time"
)

type checkpointSettings struct {
	settingsRepoStub
	values map[string]string
}

func (s *checkpointSettings) GetSetting(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *checkpointSettings) SetSetting(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

type incrementalHistoryStub struct {
	printerJobClientStub
	sinces []time.Time
	starts []int
	failAt int
}

func (c *incrementalHistoryStub) ListHistoryPageSince(_ context.Context, _ domain.Printer, start, limit int, since time.Time) ([]domain.PrinterHistoryJob, int, error) {
	c.sinces = append(c.sinces, since)
	c.starts = append(c.starts, start)
	if c.failAt > 0 && start >= c.failAt {
		return nil, 0, errors.New("connection lost")
	}
	var filtered []domain.PrinterHistoryJob
	for _, job := range c.history {
		if since.IsZero() || job.StartedAt.After(since) {
			filtered = append(filtered, job)
		}
	}
	page := filtered[min(start, len(filtered)):min(start+limit, len(filtered))]
	return page, len(page), nil // Actual Moonraker page-count semantics.
}

func TestHistoryCheckpointPaginationAndResume(t *testing.T) {
	settings := &checkpointSettings{values: make(map[string]string)}
	svc := &PrinterService{settings: settings}
	printer := domain.Printer{ID: "p", MoonrakerURL: "http://printer"}
	client := &incrementalHistoryStub{}
	for i := 250; i > 0; i-- {
		client.history = append(client.history, domain.PrinterHistoryJob{
			ExternalPrintID: fmt.Sprint(i), StartedAt: time.Unix(int64(i*10), 0), Status: domain.PrinterHistoryStatusCompleted,
		})
	}
	seen := make(map[string]bool)
	imported := 0
	importJobs := func(jobs []domain.PrinterHistoryJob) (int, error) {
		created := 0
		for _, job := range jobs {
			imported++
			if !seen[job.ExternalPrintID] {
				created++
				seen[job.ExternalPrintID] = true
			}
		}
		return created, nil
	}
	client.failAt = 100
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err == nil || n != 100 || len(settings.values) != 0 {
		t.Fatalf("failed import: created=%d error=%v checkpoint=%v", n, err, settings.values)
	}
	client.failAt = 0
	client.starts = nil
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 150 || len(client.starts) != 3 {
		t.Fatalf("resume: created=%d error=%v pages=%v", n, err, client.starts)
	}
	// Reconstructing the service must retain the boundary stored in settings.
	svc = &PrinterService{settings: settings}
	client.sinces = nil
	imported = 0
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 0 || imported != 1 || client.sinces[0].IsZero() {
		t.Fatalf("incremental: created=%d imported=%d error=%v since=%v", n, imported, err, client.sinces)
	}
	// An empty response must not move the boundary backwards on each poll.
	key := "printer_history_sync_v2:p"
	before := settings.values[key]
	client.history = nil
	if _, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || settings.values[key] != before {
		t.Fatalf("empty response changed checkpoint: %v", err)
	}
}

func TestHistoryCheckpointKeepsUnfinishedAndReconciles(t *testing.T) {
	settings := &checkpointSettings{values: make(map[string]string)}
	svc := &PrinterService{settings: settings}
	printer := domain.Printer{ID: "p", MoonrakerURL: "http://printer"}
	client := &incrementalHistoryStub{printerJobClientStub: printerJobClientStub{history: []domain.PrinterHistoryJob{
		{ExternalPrintID: "new", StartedAt: time.Unix(200, 0), Status: domain.PrinterHistoryStatusCompleted},
		{ExternalPrintID: "running", StartedAt: time.Unix(100, 0), Status: "in_progress"},
	}}}
	importJobs := func(jobs []domain.PrinterHistoryJob) (int, error) { return len(jobs), nil }
	if _, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil {
		t.Fatal(err)
	}
	client.history[1].Status = domain.PrinterHistoryStatusCompleted
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 2 || !client.sinces[len(client.sinces)-1].Equal(time.Unix(99, 0)) {
		t.Fatalf("unfinished job skipped: count=%d error=%v since=%v", n, err, client.sinces)
	}
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 1 {
		t.Fatalf("finished boundary did not advance: count=%d error=%v", n, err)
	}
	key := "printer_history_sync_v2:p"
	var saved historyCheckpoint
	if err := json.Unmarshal([]byte(settings.values[key]), &saved); err != nil {
		t.Fatal(err)
	}
	saved.FullSync = time.Now().Add(-25 * time.Hour)
	raw, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	settings.values[key] = string(raw)
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 2 || !client.sinces[len(client.sinces)-1].IsZero() {
		t.Fatalf("daily reconciliation: count=%d error=%v", n, err)
	}
	printer.MoonrakerURL = "http://replacement"
	if n, err := svc.syncHistoryPages(t.Context(), printer, client, importJobs); err != nil || n != 2 || !client.sinces[len(client.sinces)-1].IsZero() {
		t.Fatalf("replacement printer: count=%d error=%v", n, err)
	}
}

type historyPrintRepo struct {
	printRepoStub
	filters []domain.PrintJobFilter
}

func (r *historyPrintRepo) List(_ context.Context, filter domain.PrintJobFilter) ([]domain.PrintJob, error) {
	r.filters = append(r.filters, filter)
	return r.list, nil
}

func TestHistorySyncQueriesOnlyPageAndRetriesMissingWeight(t *testing.T) {
	for _, weight := range []float64{0, 20} {
		t.Run(fmt.Sprint(weight), func(t *testing.T) {
			prints := &historyPrintRepo{printRepoStub: printRepoStub{list: []domain.PrintJob{{ID: "local", ExternalPrintID: "remote", EstimatedWeightG: weight}}}}
			client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{ExternalPrintID: "remote", GcodeFilename: "part.gcode", Status: domain.PrinterHistoryStatusCompleted}}}
			calls := 0
			client.fetchMetadata = func(context.Context, domain.Printer, string) (*domain.GcodeMetadata, error) {
				calls++
				return &domain.GcodeMetadata{FilamentWeightG: 20}, nil
			}
			svc := NewPrinterService(&printerRepoStub{printer: &domain.Printer{ID: "p"}}, &spoolRepoStub{}, prints, &settingsRepoStub{})
			svc.SetJobClient(client)
			if n, err := svc.SyncPrinterHistory(t.Context(), "p", 0); err != nil || n != 0 {
				t.Fatalf("sync: count=%d error=%v", n, err)
			}
			if len(prints.filters) != 2 || prints.filters[0].Limit != 1 || len(prints.filters[1].HistoryMatches) != 1 {
				t.Fatalf("unbounded local lookup: %+v", prints.filters)
			}
			if calls != 1 || weight == 0 && prints.estimatedWeight != 20 {
				t.Fatalf("metadata calls=%d updated weight=%v", calls, prints.estimatedWeight)
			}
		})
	}
}
