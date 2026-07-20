package moonraker

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"awesomeProject/internal/domain"
)

const subscribeRequestID = 1

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int    `json:"id"`
}

type subscribeParams struct {
	Objects map[string]any `json:"objects"`
}

func SubscribeRequest() rpcRequest {
	return rpcRequest{
		JSONRPC: "2.0",
		Method:  "printer.objects.subscribe",
		Params: subscribeParams{Objects: map[string]any{
			"print_stats":    nil,
			"virtual_sdcard": nil,
			"display_status": nil,
			"webhooks":       nil,
		}},
		ID: subscribeRequestID,
	}
}

type rpcEnvelope struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
}

type StatusUpdate struct {
	State            domain.PrinterState
	RawPrintState    string
	Filename         string
	Progress         float64
	FilamentUsedMM   float64
	TotalDuration    time.Duration
	PrintDuration    time.Duration
	KlippyState      string
	KlippyStateError string
}

func DecodeStatusUpdate(raw json.RawMessage) (StatusUpdate, bool, error) {
	var envelope rpcEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return StatusUpdate{}, false, fmt.Errorf("decoding moonraker message: %w", err)
	}
	if envelope.Method != "notify_status_update" || len(envelope.Params) == 0 {
		return StatusUpdate{}, false, nil
	}

	var objects map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Params[0], &objects); err != nil {
		return StatusUpdate{}, false, fmt.Errorf("decoding moonraker status objects: %w", err)
	}

	update := StatusUpdate{State: domain.PrinterStateUnknown}

	if rawPrintStats, ok := objects["print_stats"]; ok {
		var printStats struct {
			State         string  `json:"state"`
			Filename      string  `json:"filename"`
			FilamentUsed  float64 `json:"filament_used"`
			TotalDuration float64 `json:"total_duration"`
			PrintDuration float64 `json:"print_duration"`
			Message       string  `json:"message"`
		}
		if err := json.Unmarshal(rawPrintStats, &printStats); err != nil {
			return StatusUpdate{}, false, fmt.Errorf("decoding print_stats: %w", err)
		}
		update.RawPrintState = printStats.State
		update.State = mapPrintState(printStats.State)
		update.Filename = printStats.Filename
		update.FilamentUsedMM = printStats.FilamentUsed
		update.TotalDuration = secondsDuration(printStats.TotalDuration)
		update.PrintDuration = secondsDuration(printStats.PrintDuration)
	}

	if rawVirtualSD, ok := objects["virtual_sdcard"]; ok {
		var virtualSD struct {
			Progress float64 `json:"progress"`
		}
		if err := json.Unmarshal(rawVirtualSD, &virtualSD); err != nil {
			return StatusUpdate{}, false, fmt.Errorf("decoding virtual_sdcard: %w", err)
		}
		update.Progress = normalizeProgress(virtualSD.Progress)
	}

	if rawDisplay, ok := objects["display_status"]; ok && update.Progress == 0 {
		var display struct {
			Progress float64 `json:"progress"`
		}
		if err := json.Unmarshal(rawDisplay, &display); err != nil {
			return StatusUpdate{}, false, fmt.Errorf("decoding display_status: %w", err)
		}
		update.Progress = normalizeProgress(display.Progress)
	}

	if rawWebhooks, ok := objects["webhooks"]; ok {
		var webhooks struct {
			State        string `json:"state"`
			StateMessage string `json:"state_message"`
		}
		if err := json.Unmarshal(rawWebhooks, &webhooks); err != nil {
			return StatusUpdate{}, false, fmt.Errorf("decoding webhooks: %w", err)
		}
		update.KlippyState = webhooks.State
		update.KlippyStateError = webhooks.StateMessage
		if webhooks.State == "error" || webhooks.State == "shutdown" {
			update.State = domain.PrinterStateError
		}
	}

	return update, true, nil
}

func mapPrintState(state string) domain.PrinterState {
	switch state {
	case "printing":
		return domain.PrinterStatePrinting
	case "paused":
		return domain.PrinterStatePaused
	case "standby", "complete":
		return domain.PrinterStateReady
	case "error":
		return domain.PrinterStateError
	default:
		return domain.PrinterStateUnknown
	}
}

func statusToPrintStatus(state string) (domain.PrintStatus, bool) {
	switch state {
	case "complete":
		return domain.PrintStatusSuccess, true
	case "cancelled":
		return domain.PrintStatusCancelled, true
	case "error":
		return domain.PrintStatusFailed, true
	default:
		return "", false
	}
}

func normalizeProgress(progress float64) float64 {
	if math.IsNaN(progress) || progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func secondsDuration(seconds float64) time.Duration {
	if seconds <= 0 || math.IsNaN(seconds) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}
