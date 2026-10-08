package moonraker

import (
	"awesomeProject/internal/domain"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPMetadataFetcher(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		switch r.URL.Path {
		case "/server/files/metadata":
		case "/server/files/gcodes/folder/.thumbs/benchy.png":
			_, _ = fmt.Fprint(w, "png-data")
			return
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("filename"); got != "folder/benchy.gcode" {
			t.Fatalf("filename query = %q", got)
		}
		_, _ = fmt.Fprint(w, `{"result":{
			"filament_total": 9000,
			"filament_weight_total": 27.5,
			"estimated_time": 3600,
			"layer_height": 0.2,
			"first_layer_extr_temp": 215,
			"first_layer_bed_temp": 60,
			"slicer": "OrcaSlicer",
			"thumbnails": [
				{"width": 64, "height": 64, "size": 10, "relative_path": ".thumbs/small.png"},
				{"width": 300, "height": 300, "size": 20, "relative_path": ".thumbs/benchy.png"}
			]
		}}`)
	}))
	t.Cleanup(server.Close)

	cacheDir := t.TempDir()
	metadata, err := HTTPMetadataFetcher{CacheDir: cacheDir}.Fetch(t.Context(), domain.Printer{
		ID:           "printer-1",
		MoonrakerURL: server.URL,
		APIKey:       "secret",
	}, "folder/benchy.gcode")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key header = %q", gotAPIKey)
	}
	if metadata.FilamentWeightTotal != 27.5 || metadata.FilamentTotalMM != 9000 || metadata.EstimatedTime != time.Hour || metadata.NozzleTempC != 215 || metadata.BedTempC != 60 || metadata.Slicer != "OrcaSlicer" || len(metadata.Thumbnails) != 2 {
		t.Fatalf("metadata = %+v", metadata)
	}
	if metadata.PreviewImagePath == "" || filepath.Dir(metadata.PreviewImagePath) != cacheDir {
		t.Fatalf("preview path = %q, want file in %q", metadata.PreviewImagePath, cacheDir)
	}
	contents, err := os.ReadFile(metadata.PreviewImagePath)
	if err != nil || string(contents) != "png-data" {
		t.Fatalf("cached preview = (%q, %v)", string(contents), err)
	}

	blockedCache := filepath.Join(t.TempDir(), "cache-file")
	if err := os.WriteFile(blockedCache, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err = (HTTPMetadataFetcher{CacheDir: blockedCache}).Fetch(t.Context(), domain.Printer{
		MoonrakerURL: server.URL, APIKey: "secret",
	}, "folder/benchy.gcode")
	if err != nil || metadata.PreviewImagePath != "" || metadata.PreviewImageHash == "" || string(metadata.PreviewImageData) != "png-data" {
		t.Fatalf("cache failure discarded usable preview: %+v, %v", metadata, err)
	}
}

func TestHTTPMetadataFetcherCheck(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		if r.URL.Path != "/server/info" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"result":{"klippy_state":"ready"}}`)
	}))
	t.Cleanup(server.Close)

	err := HTTPMetadataFetcher{}.Check(t.Context(), domain.Printer{
		MoonrakerURL: server.URL,
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key header = %q", gotAPIKey)
	}
}

type fakeMetadataFetcher struct {
	metadata FileMetadata
	err      error
}

func (f fakeMetadataFetcher) Fetch(context.Context, domain.Printer, string) (FileMetadata, error) {
	return f.metadata, f.err
}

func TestEnrichEventWithMetadata(t *testing.T) {
	event := &domain.PrinterPrintEvent{
		GcodeFilename:     "benchy.gcode",
		EstimatedLengthMM: 1000,
		ActualLengthMM:    100,
		PrintDuration:     0,
	}
	enrichEventWithMetadata(t.Context(), fakeMetadataFetcher{metadata: FileMetadata{
		FilamentWeightTotal: 12.3,
		FilamentTotalMM:     4567,
		EstimatedTime:       30 * time.Minute,
		NozzleTempC:         220,
		BedTempC:            65,
		Slicer:              "PrusaSlicer",
		PreviewImagePath:    "/tmp/preview.png",
	}}, domain.Printer{}, event)

	if event.ActualLengthMM != 100 || event.EstimatedWeightG != 12.3 || event.EstimatedLengthMM != 4567 || event.PrintDuration != 30*time.Minute || event.NozzleTempC != 220 || event.BedTempC != 65 || event.Slicer != "PrusaSlicer" || event.PreviewImagePath != "/tmp/preview.png" {
		t.Fatalf("enriched event = %+v", event)
	}
}
