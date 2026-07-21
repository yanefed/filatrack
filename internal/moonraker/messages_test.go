package moonraker

import (
	"encoding/json"
	"testing"
	"time"

	"awesomeProject/internal/domain"
)

func TestSubscribeRequest(t *testing.T) {
	req := SubscribeRequest()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	var decoded struct {
		Method string `json:"method"`
		Params struct {
			Objects map[string]any `json:"objects"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding request: %v", err)
	}
	if decoded.Method != "printer.objects.subscribe" {
		t.Fatalf("method = %q", decoded.Method)
	}
	for _, name := range []string{"print_stats", "virtual_sdcard", "display_status", "webhooks"} {
		if _, ok := decoded.Params.Objects[name]; !ok {
			t.Fatalf("missing subscribed object %q in %s", name, string(raw))
		}
	}
}

func TestDecodeStatusUpdate(t *testing.T) {
	raw := json.RawMessage(`{
		"jsonrpc": "2.0",
		"method": "notify_status_update",
		"params": [{
			"print_stats": {
				"state": "printing",
				"filename": "benchy.gcode",
				"filament_used": 1234.5,
				"total_duration": 42,
				"print_duration": 40
			},
			"virtual_sdcard": {"progress": 0.25},
			"display_status": {"progress": 0.20},
			"webhooks": {"state": "ready"}
		}, 123456.0]
	}`)

	update, ok, err := DecodeStatusUpdate(raw)
	if err != nil || !ok {
		t.Fatalf("DecodeStatusUpdate() = (%+v, %v, %v)", update, ok, err)
	}
	if update.State != domain.PrinterStatePrinting || update.RawPrintState != "printing" || update.Filename != "benchy.gcode" || update.Progress != 0.20 || update.FilamentUsedMM != 1234.5 || update.TotalDuration != 42*time.Second || update.PrintDuration != 40*time.Second {
		t.Fatalf("unexpected status update: %+v", update)
	}
	if !update.HasState || !update.HasFilename || !update.HasProgress {
		t.Fatalf("expected update flags to be set: %+v", update)
	}
}

func TestDecodeStatusUpdatePartialProgressOnly(t *testing.T) {
	raw := json.RawMessage(`{
		"jsonrpc": "2.0",
		"method": "notify_status_update",
		"params": [{
			"display_status": {"progress": 0.33}
		}]
	}`)
	update, ok, err := DecodeStatusUpdate(raw)
	if err != nil || !ok {
		t.Fatalf("DecodeStatusUpdate() = (%+v, %v, %v)", update, ok, err)
	}
	if update.HasState || update.HasFilename || !update.HasProgress || update.Progress != 0.33 {
		t.Fatalf("partial update flags/progress = %+v", update)
	}
}

func TestDecodeStatusUpdateNormalizesPercentProgress(t *testing.T) {
	raw := json.RawMessage(`{"jsonrpc":"2.0","method":"notify_status_update","params":[{"display_status":{"progress":42}}]}`)

	update, ok, err := DecodeStatusUpdate(raw)
	if err != nil || !ok {
		t.Fatalf("DecodeStatusUpdate = (%+v, %v, %v)", update, ok, err)
	}
	if !update.HasProgress || update.Progress != 0.42 {
		t.Fatalf("progress = %+v, want 0.42", update)
	}
}

func TestDecodeStatusUpdateSubscribeSnapshot(t *testing.T) {
	raw := json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"eventtime": 123456.0,
			"status": {
				"print_stats": {
					"state": "printing",
					"filename": "snapshot.gcode",
					"filament_used": 100,
					"print_duration": 12
				},
				"display_status": {"progress": 0.44},
				"webhooks": {"state": "ready"}
			}
		}
	}`)
	update, ok, err := DecodeStatusUpdate(raw)
	if err != nil || !ok {
		t.Fatalf("DecodeStatusUpdate() = (%+v, %v, %v)", update, ok, err)
	}
	if update.State != domain.PrinterStatePrinting || update.RawPrintState != "printing" || update.Filename != "snapshot.gcode" || update.Progress != 0.44 || update.FilamentUsedMM != 100 || update.PrintDuration != 12*time.Second {
		t.Fatalf("unexpected subscribe snapshot: %+v", update)
	}
	if !update.HasState || !update.HasFilename || !update.HasProgress {
		t.Fatalf("expected snapshot flags to be set: %+v", update)
	}
}

func TestDecodeStatusUpdateIgnoresNonStatusMessages(t *testing.T) {
	_, ok, err := DecodeStatusUpdate(json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	if err != nil || ok {
		t.Fatalf("non-status message = (%v, %v), want ignored", ok, err)
	}
}

func TestDecodeStatusUpdateMapsKlippyError(t *testing.T) {
	raw := json.RawMessage(`{
		"jsonrpc": "2.0",
		"method": "notify_status_update",
		"params": [{"webhooks": {"state": "shutdown", "state_message": "MCU lost"}}]
	}`)
	update, ok, err := DecodeStatusUpdate(raw)
	if err != nil || !ok || update.State != domain.PrinterStateError || update.KlippyStateError != "MCU lost" {
		t.Fatalf("klippy error update = (%+v, %v, %v)", update, ok, err)
	}
}
