package integration_test

import (
	"encoding/json"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/moonraker"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/service"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestIncrementalHistorySurvivesDatabaseReopen(t *testing.T) {
	var historyRequests, metadataRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server/files/metadata":
			metadataRequests.Add(1)
			if _, err := w.Write([]byte(`{"result":{"filament_weight_total":20}}`)); err != nil {
				t.Errorf("writing metadata response: %v", err)
			}
		case "/server/history/list":
			historyRequests.Add(1)
			query := r.URL.Query()
			var since float64
			if raw := query.Get("since"); raw != "" {
				parsed, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					t.Errorf("invalid history boundary %q: %v", raw, err)
					http.Error(w, "invalid since", http.StatusBadRequest)
					return
				}
				since = parsed
			}
			var start int
			if raw := query.Get("start"); raw != "" {
				parsed, err := strconv.Atoi(raw)
				if err != nil || parsed < 0 {
					t.Errorf("invalid history start %q: %v", raw, err)
					http.Error(w, "invalid start", http.StatusBadRequest)
					return
				}
				start = parsed
			}
			limit, err := strconv.Atoi(query.Get("limit"))
			if err != nil || limit <= 0 {
				t.Errorf("invalid history limit %q: %v", query.Get("limit"), err)
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			var jobs []map[string]any
			for i := 205; i > 0; i-- {
				if float64(i*100) <= since {
					continue
				}
				jobs = append(jobs, map[string]any{
					"job_id": strconv.Itoa(i), "filename": "part.gcode", "status": "completed",
					"start_time": i * 100, "end_time": i*100 + 50,
				})
			}
			jobs = jobs[min(start, len(jobs)):min(start+limit, len(jobs))]
			if err := json.NewEncoder(w).Encode(map[string]any{"count": len(jobs), "jobs": jobs}); err != nil {
				t.Errorf("encoding history response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	path := filepath.Join(t.TempDir(), "history.db")
	app := openApplication(t, path)
	t.Cleanup(func() {
		if err := app.db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})
	printer := domain.Printer{Name: "Test", MoonrakerURL: server.URL}
	if err := app.printers.CreateMoonrakerPrinter(t.Context(), &printer); err != nil {
		t.Fatal(err)
	}
	app.printers = newPrinterService(app.db, service.PrinterOptions{JobClient: moonraker.HTTPControlClient{Client: server.Client()}})
	if n, err := app.printers.SyncPrinterHistory(t.Context(), printer.ID, 0); err != nil || n != 205 {
		t.Fatalf("initial import: created=%d error=%v", n, err)
	}
	if historyRequests.Load() != 3 || metadataRequests.Load() != 205 {
		t.Fatalf("initial requests: history=%d metadata=%d", historyRequests.Load(), metadataRequests.Load())
	}
	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}
	app = openApplication(t, path)
	app.printers = newPrinterService(app.db, service.PrinterOptions{JobClient: moonraker.HTTPControlClient{Client: server.Client()}})
	historyRequests.Store(0)
	metadataRequests.Store(0)
	if n, err := app.printers.SyncPrinterHistory(t.Context(), printer.ID, 0); err != nil || n != 0 {
		t.Fatalf("repeat import: created=%d error=%v", n, err)
	}
	// The overlapping boundary job is checked again for its missing preview.
	if historyRequests.Load() != 1 || metadataRequests.Load() != 1 {
		t.Fatalf("repeat requests: history=%d metadata=%d", historyRequests.Load(), metadataRequests.Load())
	}
	jobs, err := sqlite.NewPrintJobRepo(app.db).List(t.Context(), domain.PrintJobFilter{PrinterID: &printer.ID})
	if err != nil || len(jobs) != 205 {
		t.Fatalf("persisted jobs=%d error=%v", len(jobs), err)
	}
}
