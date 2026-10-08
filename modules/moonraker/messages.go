package moonraker

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const subscribeRequestID = 1

// RpcRequest is a Moonraker JSON-RPC request.
type RpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int    `json:"id"`
}

type subscribeParams struct {
	Objects map[string]any `json:"objects"`
}

// SubscribeRequest subscribes to print statistics, progress, and Klippy state.
func SubscribeRequest() RpcRequest {
	return RpcRequest{
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
	Result  json.RawMessage   `json:"result"`
	ID      int               `json:"id"`
}

// StatusUpdate is a partial protocol update. Use the Has fields when merging
// values into a snapshot; RawPrintState and KlippyState retain server state names.
type StatusUpdate struct {
	RawPrintState    string
	Filename         string
	Progress         float64
	HasState         bool
	HasFilename      bool
	HasProgress      bool
	FilamentUsedMM   float64
	TotalDuration    time.Duration
	PrintDuration    time.Duration
	KlippyState      string
	KlippyStateError string
}

// DecodeStatusUpdate decodes subscription snapshots and status notifications.
// It returns ok=false for messages that do not contain status objects.
func DecodeStatusUpdate(raw json.RawMessage) (StatusUpdate, bool, error) {
	var envelope rpcEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return StatusUpdate{}, false, fmt.Errorf("decoding moonraker message: %w", err)
	}

	objects, ok, err := statusObjects(envelope)
	if err != nil {
		return StatusUpdate{}, false, err
	}
	if !ok {
		return StatusUpdate{}, false, nil
	}

	update, err := decodeStatusObjects(objects)
	if err != nil {
		return StatusUpdate{}, false, err
	}
	return update, true, nil
}

func statusObjects(envelope rpcEnvelope) (map[string]json.RawMessage, bool, error) {
	if envelope.Method == "notify_status_update" {
		if len(envelope.Params) == 0 {
			return nil, false, nil
		}
		var objects map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Params[0], &objects); err != nil {
			return nil, false, fmt.Errorf("decoding moonraker status objects: %w", err)
		}
		return objects, true, nil
	}

	if len(envelope.Result) == 0 {
		return nil, false, nil
	}
	var result struct {
		Status map[string]json.RawMessage `json:"status"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, false, fmt.Errorf("decoding moonraker subscribe result: %w", err)
	}
	if len(result.Status) == 0 {
		return nil, false, nil
	}
	return result.Status, true, nil
}

func decodeStatusObjects(objects map[string]json.RawMessage) (StatusUpdate, error) {
	update := StatusUpdate{}

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
			return StatusUpdate{}, fmt.Errorf("decoding print_stats: %w", err)
		}
		if printStats.State != "" {
			update.RawPrintState = printStats.State
			update.HasState = true
		}
		if printStats.Filename != "" {
			update.Filename = printStats.Filename
			update.HasFilename = true
		}
		update.FilamentUsedMM = printStats.FilamentUsed
		update.TotalDuration = secondsDuration(printStats.TotalDuration)
		update.PrintDuration = secondsDuration(printStats.PrintDuration)
	}

	if rawVirtualSD, ok := objects["virtual_sdcard"]; ok {
		var virtualSD struct {
			Progress *float64 `json:"progress"`
		}
		if err := json.Unmarshal(rawVirtualSD, &virtualSD); err != nil {
			return StatusUpdate{}, fmt.Errorf("decoding virtual_sdcard: %w", err)
		}
		if virtualSD.Progress != nil {
			update.Progress = normalizeProgress(*virtualSD.Progress)
			update.HasProgress = true
		}
	}

	if rawDisplay, ok := objects["display_status"]; ok {
		var display struct {
			Progress *float64 `json:"progress"`
		}
		if err := json.Unmarshal(rawDisplay, &display); err != nil {
			return StatusUpdate{}, fmt.Errorf("decoding display_status: %w", err)
		}
		if display.Progress != nil {
			update.Progress = normalizeProgress(*display.Progress)
			update.HasProgress = true
		}
	}

	if rawWebhooks, ok := objects["webhooks"]; ok {
		var webhooks struct {
			State        string `json:"state"`
			StateMessage string `json:"state_message"`
		}
		if err := json.Unmarshal(rawWebhooks, &webhooks); err != nil {
			return StatusUpdate{}, fmt.Errorf("decoding webhooks: %w", err)
		}
		update.KlippyState = webhooks.State
		update.KlippyStateError = webhooks.StateMessage
	}

	return update, nil
}

func normalizeProgress(progress float64) float64 {
	if math.IsNaN(progress) || progress < 0 {
		return 0
	}
	if progress > 1 && progress <= 100 {
		return progress / 100
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

// DecodeFinishedHistoryJob reads the authoritative job identity and final data
// from notify_history_changed. Status updates alone do not identify a job.
func DecodeFinishedHistoryJob(raw json.RawMessage) (HistoryJob, bool, error) {
	var envelope rpcEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return HistoryJob{}, false, fmt.Errorf("decoding moonraker message: %w", err)
	}
	if envelope.Method != "notify_history_changed" || len(envelope.Params) == 0 {
		return HistoryJob{}, false, nil
	}
	var notification struct {
		Action string     `json:"action"`
		Job    historyJob `json:"job"`
	}
	if err := json.Unmarshal(envelope.Params[0], &notification); err != nil {
		return HistoryJob{}, false, fmt.Errorf("decoding history notification: %w", err)
	}
	if notification.Action != "finished" {
		return HistoryJob{}, false, nil
	}
	if notification.Job.JobID == "" {
		return HistoryJob{}, false, fmt.Errorf("finished history job has no job_id")
	}
	return notification.Job.decode(), true, nil
}
