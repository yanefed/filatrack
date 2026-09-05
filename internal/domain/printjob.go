package domain

import (
	"time"
)

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

func CreatePrinterJobFromPrinterEvent(printer Printer, printEvent PrinterPrintEvent, spool Spool) *PrintJob {
	status := printEvent.getStatus()
	completion := printEvent.getCompletionPercent()
	estimatedWeight := printEvent.getEstimatedWeightG(spool)
	actualWeight := printEvent.getActualWeightG(estimatedWeight, completion)
	// Persist consumed length, as history imports and spool reassignment expect.
	usedLength := printEvent.EstimatedLengthMM * float64(completion) / 100
	if printEvent.ActualLengthMM > 0 {
		usedLength = printEvent.ActualLengthMM
		if printEvent.ActualWeightG == 0 {
			actualWeight = WeightFromLength(usedLength, spool.DiameterMM, spool.DensityGCM3)
		}
	}

	return &PrintJob{
		SpoolID:           printer.ActiveSpoolID,
		PrinterID:         printer.ID,
		ExternalPrintID:   printEvent.ExternalPrintID,
		GcodeFilename:     printEvent.GcodeFilename,
		ModelName:         printEvent.getModelName(),
		Status:            status,
		CompletionPercent: completion,
		EstimatedWeightG:  estimatedWeight,
		ActualWeightG:     actualWeight,
		EstimatedLengthMM: usedLength,
		PrintDuration:     printEvent.PrintDuration,
		NozzleTempC:       printEvent.NozzleTempC,
		BedTempC:          printEvent.BedTempC,
		Slicer:            printEvent.Slicer,
		PreviewImagePath:  printEvent.PreviewImagePath,
		PrintedAt:         printEvent.getPrintedAt(),
		CreatedAt:         time.Now(),
	}
}

func CreatePrinterJobFromPrinterHistoryItem(item PrinterHistoryJob, spool *Spool, status PrintStatus) *PrintJob {
	estimatedWeight := item.getEstimatedWeightG(spool)
	actualWeight := item.getActualWeightG(spool)
	completion := item.getCompletionPercent(status, estimatedWeight, actualWeight)

	if actualWeight == 0 && status == PrintStatusSuccess {
		actualWeight = estimatedWeight
	}

	spoolID := ""
	if spool != nil {
		spoolID = spool.ID
	}

	return &PrintJob{
		SpoolID:           spoolID,
		PrinterID:         item.PrinterID,
		ExternalPrintID:   item.ExternalPrintID,
		GcodeFilename:     item.GcodeFilename,
		ModelName:         item.getModelName(),
		Status:            status,
		CompletionPercent: completion,
		EstimatedWeightG:  estimatedWeight,
		ActualWeightG:     actualWeight,
		EstimatedLengthMM: item.FilamentUsedMM,
		PrintDuration:     item.PrintDuration,
		NozzleTempC:       item.NozzleTempC,
		BedTempC:          item.BedTempC,
		Slicer:            item.Slicer,
		PreviewImagePath:  item.PreviewImagePath,
		PrintedAt:         item.getEndedAt(),
		CreatedAt:         time.Now(),
	}
}
