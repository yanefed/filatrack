package integration_test

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/moonraker"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/repository/sqlite"
	"encoding/json"
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
			_, _ = w.Write([]byte(`{"result":{"filament_weight_total":20}}`))
		case "/server/history/list":
			historyRequests.Add(1)
			query := r.URL.Query()
			since, _ := strconv.ParseFloat(query.Get("since"), 64)
			start, _ := strconv.Atoi(query.Get("start"))
			limit, _ := strconv.Atoi(query.Get("limit"))
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
			_ = json.NewEncoder(w).Encode(map[string]any{"count": len(jobs), "jobs": jobs})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	path := filepath.Join(t.TempDir(), "history.db")
	app := openApplication(t, path)
	t.Cleanup(func() { _ = app.db.Close() })
	printer := domain.Printer{Name: "Test", MoonrakerURL: server.URL}
	if err := app.printers.CreateMoonrakerPrinter(t.Context(), &printer); err != nil {
		t.Fatal(err)
	}
	app.printers.SetJobClient(moonraker.HTTPControlClient{Client: server.Client()})
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
	app.printers.SetJobClient(moonraker.HTTPControlClient{Client: server.Client()})
	historyRequests.Store(0)
	metadataRequests.Store(0)
	if n, err := app.printers.SyncPrinterHistory(t.Context(), printer.ID, 0); err != nil || n != 0 {
		t.Fatalf("repeat import: created=%d error=%v", n, err)
	}
	if historyRequests.Load() != 1 || metadataRequests.Load() != 0 {
		t.Fatalf("repeat requests: history=%d metadata=%d", historyRequests.Load(), metadataRequests.Load())
	}
	jobs, err := sqlite.NewPrintJobRepo(app.db).List(t.Context(), repository.PrintJobFilter{PrinterID: &printer.ID})
	if err != nil || len(jobs) != 205 {
		t.Fatalf("persisted jobs=%d error=%v", len(jobs), err)
	}
}
