package moonraker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestThumbnailURL(t *testing.T) {
	got, err := thumbnailURL("ws://printer.local/websocket", "folder/part.gcode", ".thumbs/part.png")
	if err != nil {
		t.Fatalf("thumbnailURL: %v", err)
	}
	want := "http://printer.local/server/files/gcodes/folder/.thumbs/part.png"
	if got != want {
		t.Fatalf("thumbnail URL = %q, want %q", got, want)
	}
}

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

	metadata, err := Client{}.FetchMetadata(t.Context(), Config{
		URL:    server.URL,
		APIKey: "secret",
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
	if string(metadata.ThumbnailData) != "png-data" {
		t.Fatalf("thumbnail = %q", metadata.ThumbnailData)
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

	err := Client{}.Check(t.Context(), Config{
		URL:    server.URL,
		APIKey: "secret",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("API key header = %q", gotAPIKey)
	}
}
