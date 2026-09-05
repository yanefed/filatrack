package domain

import (
	"time"
)

type PrinterHistoryStatus string

const (
	PrinterHistoryStatusCompleted        PrinterHistoryStatus = "completed"
	PrinterHistoryStatusSuccess          PrinterHistoryStatus = "success"
	PrinterHistoryStatusCancelled        PrinterHistoryStatus = "cancelled"
	PrinterHistoryStatusError            PrinterHistoryStatus = "error"
	PrinterHistoryStatusKlippyShutdown   PrinterHistoryStatus = "klippy_shutdown"
	PrinterHistoryStatusKlippyDisconnect PrinterHistoryStatus = "klippy_disconnect"
	PrinterHistoryStatusInterrupted      PrinterHistoryStatus = "interrupted"
)

type PrinterHistoryJob struct {
	PrinterID        string
	ExternalPrintID  string
	GcodeFilename    string
	ModelName        string
	Status           PrinterHistoryStatus
	FilamentUsedMM   float64
	EstimatedWeightG float64
	PrintDuration    time.Duration
	TotalDuration    time.Duration
	NozzleTempC      int
	BedTempC         int
	Slicer           string
	PreviewImagePath string
	StartedAt        time.Time
	EndedAt          time.Time
	LocalPrintJobID  string
	LocalSpoolID     string
}

type PrinterModelFile struct {
	PrinterID        string
	Path             string
	Filename         string
	ModifiedAt       time.Time
	SizeBytes        int64
	EstimatedWeightG float64
	PrintDuration    time.Duration
	Slicer           string
	PreviewImagePath string
	CachedAt         time.Time
}

func (j *PrinterHistoryJob) PrintStatus() (PrintStatus, bool) {
	switch j.Status {
	case PrinterHistoryStatusCompleted, PrinterHistoryStatusSuccess:
		return PrintStatusSuccess, true
	case PrinterHistoryStatusCancelled:
		return PrintStatusCancelled, true
	case PrinterHistoryStatusError, PrinterHistoryStatusKlippyShutdown, PrinterHistoryStatusKlippyDisconnect, PrinterHistoryStatusInterrupted:
		return PrintStatusFailed, true
	default:
		return "", false
	}
}

func (j *PrinterHistoryJob) getEstimatedWeightG(spool *Spool) float64 {
	if j.EstimatedWeightG == 0 && j.FilamentUsedMM > 0 && spool != nil &&
		(j.Status == PrinterHistoryStatusCompleted || j.Status == PrinterHistoryStatusSuccess) {
		return WeightFromLength(j.FilamentUsedMM, spool.DiameterMM, spool.DensityGCM3)
	}
	return j.EstimatedWeightG
}

func (j *PrinterHistoryJob) getActualWeightG(spool *Spool) float64 {
	if j.FilamentUsedMM > 0 && spool != nil {
		return WeightFromLength(j.FilamentUsedMM, spool.DiameterMM, spool.DensityGCM3)
	}
	return 0
}

func (j *PrinterHistoryJob) getCompletionPercent(status PrintStatus, estimatedWeight, actualWeight float64) int {
	completion := 100
	if status != PrintStatusSuccess {
		completion = 0
		if estimatedWeight > 0 && actualWeight > 0 {
			completion = min(int(actualWeight/estimatedWeight*100), 100)
		}
	}
	return completion
}

func (j *PrinterHistoryJob) getEndedAt() time.Time {
	if j.EndedAt.IsZero() {
		return time.Now()
	}
	return j.EndedAt
}

func (j *PrinterHistoryJob) getModelName() string {
	if j.ModelName == "" {
		return modelNameFromGcode(j.GcodeFilename)
	}
	return j.ModelName
}

type FilamentCheck struct {
	RequiredWeightG  float64
	RemainingBeforeG float64
	RemainingAfterG  float64
	Warning          string
	Blocked          bool
}

type PrintLaunchRequest struct {
	PrinterID        string
	SourcePrinterID  string
	SpoolID          string
	GcodeFilename    string
	LocalPath        string
	AllowLowFilament bool
}

type PrinterUploadResult struct {
	Path         string
	PrintStarted bool
	PrintQueued  bool
}

type PrintLaunchResult struct {
	Filename                 string
	Uploaded                 bool
	Started                  bool
	Queued                   bool
	Filament                 FilamentCheck
	RequiresUserConfirmation bool
}
