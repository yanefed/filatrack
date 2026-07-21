package domain

import "time"

type PrintStatus string

const (
	PrintStatusSuccess   PrintStatus = "success"
	PrintStatusFailed    PrintStatus = "failed"
	PrintStatusCancelled PrintStatus = "cancelled"
)

type PrintJob struct {
	ID                string
	SpoolID           string
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
	Notes             string
	PrintedAt         time.Time
	CreatedAt         time.Time
}

func (p *PrintJob) CalculateActualWeight() float64 {
	return p.EstimatedWeightG * float64(p.CompletionPercent) / 100
}
