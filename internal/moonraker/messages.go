package moonraker

import (
	"encoding/json"
	"time"

	"github.com/yanefed/filatrack/internal/domain"

	api "github.com/yanefed/moonraker"
)

type RpcRequest = api.RpcRequest

func SubscribeRequest() RpcRequest { return api.SubscribeRequest() }

// StatusUpdate adds Filatrack's state mapping to a protocol update.
type StatusUpdate struct {
	State            domain.PrinterState
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

func DecodeStatusUpdate(raw json.RawMessage) (StatusUpdate, bool, error) {
	update, ok, err := api.DecodeStatusUpdate(raw)
	if err != nil || !ok {
		return StatusUpdate{}, ok, err
	}
	state := mapPrintState(update.RawPrintState)
	if update.KlippyState == "error" || update.KlippyState == "shutdown" {
		state = domain.PrinterStateError
	}
	return StatusUpdate{
		State: state, RawPrintState: update.RawPrintState, Filename: update.Filename,
		Progress: update.Progress, HasState: update.HasState, HasFilename: update.HasFilename,
		HasProgress: update.HasProgress, FilamentUsedMM: update.FilamentUsedMM,
		TotalDuration: update.TotalDuration, PrintDuration: update.PrintDuration,
		KlippyState: update.KlippyState, KlippyStateError: update.KlippyStateError,
	}, true, nil
}

func mapPrintState(state string) domain.PrinterState {
	switch state {
	case "printing":
		return domain.PrinterStatePrinting
	case "paused":
		return domain.PrinterStatePaused
	case "standby", "complete", "cancelled":
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
