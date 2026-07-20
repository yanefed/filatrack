package domain

import "time"

type PrinterKind string

const PrinterKindMoonraker PrinterKind = "moonraker"

type PrinterState string

const (
	PrinterStateUnknown  PrinterState = "unknown"
	PrinterStateReady    PrinterState = "ready"
	PrinterStatePrinting PrinterState = "printing"
	PrinterStatePaused   PrinterState = "paused"
	PrinterStateError    PrinterState = "error"
	PrinterStateOffline  PrinterState = "offline"
)

type Printer struct {
	ID            string
	Name          string
	Kind          PrinterKind
	MoonrakerURL  string
	APIKey        string
	Enabled       bool
	ActiveSpoolID string
	State         PrinterState
	CurrentFile   string
	Progress      float64
	LastSeenAt    time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PrinterPrintEvent struct {
	PrinterID         string
	ExternalPrintID   string
	GcodeFilename     string
	ModelName         string
	Status            PrintStatus
	CompletionPercent int
	EstimatedWeightG  float64
	ActualWeightG     float64
	EstimatedLengthMM float64
	PrintDuration     time.Duration
	NozzleTempC       int
	BedTempC          int
	Slicer            string
	PrintedAt         time.Time
}
