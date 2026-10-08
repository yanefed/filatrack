package moonraker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	moonraker "github.com/yanefed/moonraker"
)

func TestWebSocketStatusSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/websocket" || r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("unexpected handshake: %s, API key %q", r.URL, r.Header.Get("X-Api-Key"))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		var request moonraker.RpcRequest
		if err := conn.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Method != "printer.objects.subscribe" {
			t.Errorf("method = %q", request.Method)
		}
		if err := conn.WriteJSON(json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"status":{"print_stats":{"state":"printing","filename":"part.gcode"},"virtual_sdcard":{"progress":0.25}}}}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	conn, err := (moonraker.WebSocketDialer{}).Dial(t.Context(), moonraker.Config{URL: server.URL, APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(moonraker.SubscribeRequest()); err != nil {
		t.Fatal(err)
	}
	var raw json.RawMessage
	if err := conn.ReadJSON(&raw); err != nil {
		t.Fatal(err)
	}
	update, ok, err := moonraker.DecodeStatusUpdate(raw)
	if err != nil || !ok || update.RawPrintState != "printing" || update.Filename != "part.gcode" || !update.HasProgress || update.Progress != 0.25 {
		t.Fatalf("subscription snapshot = %+v, %v, %v", update, ok, err)
	}
}

func TestDownloadFileAndRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("missing API key")
		}
		switch r.URL.Path {
		case "/server/files/gcodes/folder/part (2).gcode":
			_, _ = fmt.Fprint(w, "G28\n")
		default:
			http.Error(w, "offline", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	client := moonraker.Client{HTTPClient: server.Client()}
	config := moonraker.Config{URL: server.URL, APIKey: "secret"}
	localPath := filepath.Join(t.TempDir(), "download.gcode")
	if err := client.DownloadFile(t.Context(), config, "folder/part (2).gcode", localPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(localPath)
	if err != nil || string(data) != "G28\n" {
		t.Fatalf("download = %q, %v", data, err)
	}
	if err := client.Check(t.Context(), config); err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("Check error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := client.Check(ctx, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request = %v", err)
	}
}

func TestMetadataSurvivesMissingThumbnail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/server/files/metadata" {
			_, _ = fmt.Fprint(w, `{"result":{"filament_weight_total":12.5,"thumbnails":[{"width":300,"height":300,"relative_path":".thumbs/part.png"}]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	metadata, err := (moonraker.Client{HTTPClient: server.Client()}).FetchMetadata(t.Context(), moonraker.Config{URL: server.URL}, "part.gcode")
	if err != nil || metadata.FilamentWeightTotal != 12.5 || len(metadata.ThumbnailData) != 0 || len(metadata.Thumbnails) != 1 {
		t.Fatalf("metadata = %+v, %v", metadata, err)
	}
}
