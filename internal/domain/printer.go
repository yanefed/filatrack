package domain

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

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
	PreviewImagePath  string
	PrintedAt         time.Time
}

func (e *PrinterPrintEvent) getStatus() PrintStatus {
	if e.Status == "" {
		return PrintStatusSuccess
	}
	return e.Status
}

func (e *PrinterPrintEvent) getCompletionPercent() int {
	if e.Status == PrintStatusSuccess || e.CompletionPercent <= 0 {
		return 100
	}
	return e.CompletionPercent
}

func (e *PrinterPrintEvent) getEstimatedWeightG(spool Spool) float64 {
	if e.EstimatedWeightG == 0 && e.EstimatedLengthMM > 0 {
		return WeightFromLength(e.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
	}
	return e.EstimatedWeightG
}

func (e *PrinterPrintEvent) getActualWeightG(estimatedWeight float64, completionPercent int) float64 {
	if e.ActualWeightG == 0 {
		return estimatedWeight * float64(completionPercent) / 100
	}
	return e.ActualWeightG
}

func (e *PrinterPrintEvent) getPrintedAt() time.Time {
	if e.PrintedAt.IsZero() {
		return time.Now()
	}
	return e.PrintedAt
}

func (e *PrinterPrintEvent) getModelName() string {
	if e.ModelName == "" {
		return modelNameFromGcode(e.GcodeFilename)
	}
	return e.ModelName
}

func modelNameFromGcode(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func (p *Printer) ValidateMoonrakerURL() error {
	parsed, err := url.Parse(strings.TrimSpace(p.MoonrakerURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("valid moonraker URL is required")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("moonraker URL must use http, https, ws, or wss")
	}
	return nil
}
