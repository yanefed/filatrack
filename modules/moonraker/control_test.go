package moonraker

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClientHistorySinceAndPageCounts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		query := r.URL.Query()
		if query.Get("since") != "1000.500000000" || query.Get("order") != "desc" || r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		_, _ = fmt.Fprint(w, `{"count":1,"jobs":[{"job_id":"p","start_time":1001,"status":"in_progress"}]}`)
	}))
	t.Cleanup(server.Close)
	client := Client{HTTPClient: server.Client()}
	jobs, count, err := client.ListHistoryPageSince(t.Context(), Config{URL: server.URL, APIKey: "secret"}, 100, 100, time.Unix(1000, 500000000))
	if err != nil || count != 1 || len(jobs) != 1 || jobs[0].StartedAt != time.Unix(1001, 0) || requests != 1 {
		t.Fatalf("history page: jobs=%+v count=%d error=%v", jobs, count, err)
	}
}

func TestClientListHistoryWithCountPerPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		size := min(100, 205-start)
		_, _ = fmt.Fprintf(w, `{"count":%d,"jobs":[`, size)
		for i := 0; i < size; i++ {
			if i > 0 {
				_, _ = fmt.Fprint(w, ",")
			}
			_, _ = fmt.Fprintf(w, `{"job_id":"%d","status":"completed"}`, start+i)
		}
		_, _ = fmt.Fprint(w, "]}")
	}))
	t.Cleanup(server.Close)
	jobs, err := (Client{HTTPClient: server.Client()}).ListHistory(t.Context(), Config{URL: server.URL}, 0)
	if err != nil || len(jobs) != 205 || requests != 3 {
		t.Fatalf("history: jobs=%d requests=%d error=%v", len(jobs), requests, err)
	}
}

func TestClientListHistory(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		if r.URL.Path != "/server/history/list" || r.URL.Query().Get("limit") != "25" || r.URL.Query().Get("order") != "desc" {
			t.Fatalf("history request URL = %s", r.URL.String())
		}
		_, _ = fmt.Fprint(w, `{"jobs":[{
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

	jobs, err := Client{}.ListHistory(t.Context(), Config{URL: server.URL, APIKey: "secret"}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key = %q", gotAPIKey)
	}
	if len(jobs) != 1 || jobs[0].JobID != "job-1" || jobs[0].EstimatedWeightG != 12.5 || jobs[0].PrintDuration != time.Minute || jobs[0].NozzleTempC != 215 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestClientListHistoryFromResultObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"result":{"count":1,"jobs":[{
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

	jobs, err := Client{}.ListHistory(t.Context(), Config{URL: server.URL}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "job-2" || jobs[0].Filename != "part.gcode" || jobs[0].EstimatedWeightG != 5.1 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestClientListHistoryParsesFlexibleFilamentUsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"result":{"count":2,"jobs":[
			{"job_id":"job-1","filename":"one.gcode","status":"cancelled","filament_used":"1234","end_time":1000},
			{"job_id":"job-2","filename":"two.gcode","status":"cancelled","metadata":{"filament_used":5678},"end_time":900}
		]}}`)
	}))
	t.Cleanup(server.Close)

	jobs, err := Client{}.ListHistory(t.Context(), Config{URL: server.URL}, 25)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(jobs) != 2 || jobs[0].FilamentUsedMM != 1234 || jobs[1].FilamentUsedMM != 5678 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestClientListHistoryFetchesAllPages(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/server/history/list" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("order") != "desc" {
			t.Fatalf("history request URL = %s", r.URL.String())
		}
		switch r.URL.Query().Get("start") {
		case "":
			_, _ = fmt.Fprint(w, `{"result":{"count":2,"jobs":[{"job_id":"job-1","filename":"one.gcode","status":"completed","end_time":1000}]}}`)
		case "1":
			_, _ = fmt.Fprint(w, `{"result":{"count":2,"jobs":[{"job_id":"job-2","filename":"two.gcode","status":"completed","end_time":900}]}}`)
		default:
			t.Fatalf("unexpected start query: %q", r.URL.Query().Get("start"))
		}
	}))
	t.Cleanup(server.Close)

	jobs, err := Client{}.ListHistory(t.Context(), Config{URL: server.URL}, 0)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if requests != 2 || len(jobs) != 2 || jobs[0].JobID != "job-1" || jobs[1].JobID != "job-2" {
		t.Fatalf("requests=%d jobs=%+v", requests, jobs)
	}
}

func TestClientStartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/printer/print/start" || r.URL.Query().Get("filename") != "folder/part.gcode" {
			t.Fatalf("start request = %s %s", r.Method, r.URL.String())
		}
		_, _ = fmt.Fprint(w, `"ok"`)
	}))
	t.Cleanup(server.Close)

	if err := (Client{}).StartFile(t.Context(), Config{URL: server.URL}, "folder/part.gcode"); err != nil {
		t.Fatalf("StartFile: %v", err)
	}
}

func TestClientListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/files/list" || r.URL.Query().Get("root") != "gcodes" {
			t.Fatalf("file list request URL = %s", r.URL.String())
		}
		_, _ = fmt.Fprint(w, `{"result":[
			{"path":"folder/cube.gcode","type":"file","size":1234,"modified":1000},
			{"path":"folder","type":"directory","size":0,"modified":1000}
		]}`)
	}))
	t.Cleanup(server.Close)

	models, err := Client{}.ListFiles(t.Context(), Config{URL: server.URL})
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(models) != 1 || models[0].Path != "folder/cube.gcode" || models[0].Filename != "cube.gcode" || models[0].SizeBytes != 1234 || models[0].ModifiedAt != time.Unix(1000, 0) {
		t.Fatalf("models = %+v", models)
	}
}

func TestClientUpload(t *testing.T) {
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
		defer func(file multipart.File) {
			fileErr := file.Close()
			if fileErr != nil {
				t.Errorf("close file: %v", fileErr)
			}
		}(file)
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("reading uploaded file: %v", err)
		}
		if header.Filename != "part.gcode" || !strings.Contains(string(data), "G28") {
			t.Fatalf("uploaded file = %q %q", header.Filename, string(data))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"item":{"path":"part.gcode"},"print_started":true,"print_queued":false}`)
	}))
	t.Cleanup(server.Close)

	uploaded, err := Client{}.Upload(t.Context(), Config{URL: server.URL}, path, "part.gcode", true)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if uploaded.Path != "part.gcode" || !uploaded.PrintStarted || uploaded.PrintQueued {
		t.Fatalf("uploaded = %+v", uploaded)
	}
}

func TestClientUploadToNestedPath(t *testing.T) {
	localPath := filepath.Join(t.TempDir(), "source.gcode")
	if err := os.WriteFile(localPath, []byte("G28\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		_, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		if header.Filename != "part (2).gcode" || r.FormValue("path") != "models/folder" || r.FormValue("print") != "true" {
			t.Errorf("upload filename=%q path=%q print=%q", header.Filename, r.FormValue("path"), r.FormValue("print"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"item":{"path":"models/folder/part (2).gcode"},"print_started":true}`)
	}))
	defer server.Close()
	result, err := (Client{}).Upload(t.Context(), Config{URL: server.URL}, localPath, "models/folder/part (2).gcode", true)
	if err != nil || result.Path != "models/folder/part (2).gcode" {
		t.Fatalf("Upload = %+v, %v", result, err)
	}
}
