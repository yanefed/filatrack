package moonraker

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awesomeProject/internal/domain"
)

func TestHTTPControlClientListHistory(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		if r.URL.Path != "/server/history/list" || r.URL.Query().Get("limit") != "25" || r.URL.Query().Get("order") != "desc" {
			t.Fatalf("history request URL = %s", r.URL.String())
		}
		fmt.Fprint(w, `{"jobs":[{
			"job_id":"job-1",
			"filename":"folder/benchy.gcode",
			"status":"completed",
			"filament_used":1234,
			"print_duration":60,
			"total_duration":75,
			"start_time":1000,
			"end_time":1100,
			"metadata":{
				"filament_weight_total":12.5,
				"first_layer_extr_temp":215,
				"first_layer_bed_temp":60,
				"slicer":"OrcaSlicer"
			}
		}]}`)
	}))
	t.Cleanup(server.Close)

	jobs, err := HTTPControlClient{}.ListHistory(t.Context(), domain.Printer{ID: "printer-1", MoonrakerURL: server.URL, APIKey: "secret"}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key = %q", gotAPIKey)
	}
	if len(jobs) != 1 || jobs[0].ExternalPrintID != "job-1" || jobs[0].ModelName != "benchy" || jobs[0].EstimatedWeightG != 12.5 || jobs[0].PrintDuration != time.Minute || jobs[0].NozzleTempC != 215 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestHTTPControlClientListHistoryFromResultObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":{"count":1,"jobs":[{
			"job_id":"job-2",
			"filename":"part.gcode",
			"status":"completed",
			"filament_used":500,
			"print_duration":30,
			"total_duration":35,
			"start_time":2000,
			"end_time":2035,
			"metadata":{"filament_weight_total":5.1}
		}]}}`)
	}))
	t.Cleanup(server.Close)

	jobs, err := HTTPControlClient{}.ListHistory(t.Context(), domain.Printer{ID: "printer-1", MoonrakerURL: server.URL}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ExternalPrintID != "job-2" || jobs[0].GcodeFilename != "part.gcode" || jobs[0].EstimatedWeightG != 5.1 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestHTTPControlClientListHistoryParsesFlexibleFilamentUsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":{"count":2,"jobs":[
			{"job_id":"job-1","filename":"one.gcode","status":"cancelled","filament_used":"1234","end_time":1000},
			{"job_id":"job-2","filename":"two.gcode","status":"cancelled","metadata":{"filament_used":5678},"end_time":900}
		]}}`)
	}))
	t.Cleanup(server.Close)

	jobs, err := HTTPControlClient{}.ListHistory(t.Context(), domain.Printer{ID: "printer-1", MoonrakerURL: server.URL}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(jobs) != 2 || jobs[0].FilamentUsedMM != 1234 || jobs[1].FilamentUsedMM != 5678 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestHTTPControlClientListHistoryFetchesAllPages(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/server/history/list" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("order") != "desc" {
			t.Fatalf("history request URL = %s", r.URL.String())
		}
		switch r.URL.Query().Get("start") {
		case "":
			fmt.Fprint(w, `{"result":{"count":2,"jobs":[{"job_id":"job-1","filename":"one.gcode","status":"completed","end_time":1000}]}}`)
		case "1":
			fmt.Fprint(w, `{"result":{"count":2,"jobs":[{"job_id":"job-2","filename":"two.gcode","status":"completed","end_time":900}]}}`)
		default:
			t.Fatalf("unexpected start query: %q", r.URL.Query().Get("start"))
		}
	}))
	t.Cleanup(server.Close)

	jobs, err := HTTPControlClient{}.ListHistory(t.Context(), domain.Printer{ID: "printer-1", MoonrakerURL: server.URL}, 0)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if requests != 2 || len(jobs) != 2 || jobs[0].ExternalPrintID != "job-1" || jobs[1].ExternalPrintID != "job-2" {
		t.Fatalf("requests=%d jobs=%+v", requests, jobs)
	}
}

func TestHTTPControlClientStartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/printer/print/start" || r.URL.Query().Get("filename") != "folder/part.gcode" {
			t.Fatalf("start request = %s %s", r.Method, r.URL.String())
		}
		fmt.Fprint(w, `"ok"`)
	}))
	t.Cleanup(server.Close)

	if err := (HTTPControlClient{}).StartFile(t.Context(), domain.Printer{MoonrakerURL: server.URL}, "folder/part.gcode"); err != nil {
		t.Fatalf("StartFile: %v", err)
	}
}

func TestHTTPControlClientListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/files/list" || r.URL.Query().Get("root") != "gcodes" {
			t.Fatalf("file list request URL = %s", r.URL.String())
		}
		fmt.Fprint(w, `{"result":[
			{"path":"folder/cube.gcode","type":"file","size":1234,"modified":1000},
			{"path":"folder","type":"directory","size":0,"modified":1000}
		]}`)
	}))
	t.Cleanup(server.Close)

	models, err := HTTPControlClient{}.ListModels(t.Context(), domain.Printer{ID: "printer-1", MoonrakerURL: server.URL})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].Path != "folder/cube.gcode" || models[0].Filename != "cube.gcode" || models[0].SizeBytes != 1234 || models[0].ModifiedAt != time.Unix(1000, 0) {
		t.Fatalf("models = %+v", models)
	}
}

func TestHTTPControlClientUploadAndStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "part.gcode")
	if err := os.WriteFile(path, []byte("G28\n"), 0o644); err != nil {
		t.Fatalf("writing gcode: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/server/files/upload" {
			t.Fatalf("upload request = %s %s", r.Method, r.URL.String())
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if r.FormValue("root") != "gcodes" || r.FormValue("print") != "true" {
			t.Fatalf("form values: root=%q print=%q", r.FormValue("root"), r.FormValue("print"))
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("reading uploaded file: %v", err)
		}
		if header.Filename != "part.gcode" || !strings.Contains(string(data), "G28") {
			t.Fatalf("uploaded file = %q %q", header.Filename, string(data))
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"item":{"path":"part.gcode"},"print_started":true,"print_queued":false}`)
	}))
	t.Cleanup(server.Close)

	uploaded, err := HTTPControlClient{}.UploadAndStart(t.Context(), domain.Printer{MoonrakerURL: server.URL}, path)
	if err != nil {
		t.Fatalf("UploadAndStart: %v", err)
	}
	if uploaded.Path != "part.gcode" || !uploaded.PrintStarted || uploaded.PrintQueued {
		t.Fatalf("uploaded = %+v", uploaded)
	}
}
