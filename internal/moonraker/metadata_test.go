package moonraker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"awesomeProject/internal/domain"
)

func TestMetadataURL(t *testing.T) {
	got, err := metadataURL("ws://printer.local/websocket", "folder/part.gcode")
	if err != nil {
		t.Fatalf("metadataURL: %v", err)
	}
	want := "http://printer.local/server/files/metadata?filename=folder%2Fpart.gcode"
	if got != want {
		t.Fatalf("metadata URL = %q, want %q", got, want)
	}
}

func TestServerInfoURL(t *testing.T) {
	got, err := serverInfoURL("wss://printer.local/websocket")
	if err != nil {
		t.Fatalf("serverInfoURL: %v", err)
	}
	if got != "https://printer.local/server/info" {
		t.Fatalf("server info URL = %q", got)
	}
}

func TestHTTPMetadataFetcher(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		if got := r.URL.Query().Get("filename"); got != "folder/benchy.gcode" {
			t.Fatalf("filename query = %q", got)
		}
		fmt.Fprint(w, `{"result":{
			"filament_total": 9000,
			"filament_weight_total": 27.5,
			"estimated_time": 3600,
			"layer_height": 0.2,
			"first_layer_extr_temp": 215,
			"first_layer_bed_temp": 60,
			"slicer": "OrcaSlicer"
		}}`)
	}))
	t.Cleanup(server.Close)

	metadata, err := HTTPMetadataFetcher{}.Fetch(t.Context(), domain.Printer{
		MoonrakerURL: server.URL,
		APIKey:       "secret",
	}, "folder/benchy.gcode")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key header = %q", gotAPIKey)
	}
	if metadata.FilamentWeightTotal != 27.5 || metadata.FilamentTotalMM != 9000 || metadata.EstimatedTime != time.Hour || metadata.NozzleTempC != 215 || metadata.BedTempC != 60 || metadata.Slicer != "OrcaSlicer" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestHTTPMetadataFetcherCheck(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		if r.URL.Path != "/server/info" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"result":{"klippy_state":"ready"}}`)
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
		PrintDuration:     0,
	}
	enrichEventWithMetadata(t.Context(), fakeMetadataFetcher{metadata: FileMetadata{
		FilamentWeightTotal: 12.3,
		FilamentTotalMM:     4567,
		EstimatedTime:       30 * time.Minute,
		NozzleTempC:         220,
		BedTempC:            65,
		Slicer:              "PrusaSlicer",
	}}, domain.Printer{}, event)

	if event.EstimatedWeightG != 12.3 || event.EstimatedLengthMM != 4567 || event.PrintDuration != 30*time.Minute || event.NozzleTempC != 220 || event.BedTempC != 65 || event.Slicer != "PrusaSlicer" {
		t.Fatalf("enriched event = %+v", event)
	}
}
