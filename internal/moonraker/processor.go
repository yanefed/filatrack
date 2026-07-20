package moonraker

import (
	"fmt"
	"time"

	"awesomeProject/internal/domain"
)

type Processor struct {
	printerID string
	active    activePrint
}

type activePrint struct {
	filename        string
	startedAt       time.Time
	highestProgress float64
}

func NewProcessor(printerID string) *Processor {
	return &Processor{printerID: printerID}
}

func (p *Processor) Apply(update StatusUpdate, now time.Time) (domain.Printer, *domain.PrinterPrintEvent) {
	printer := domain.Printer{
		ID:          p.printerID,
		State:       update.State,
		CurrentFile: update.Filename,
		Progress:    update.Progress,
		LastSeenAt:  now,
	}

	if update.Filename != "" && (update.RawPrintState == "printing" || update.RawPrintState == "paused") {
		if p.active.filename != update.Filename {
			p.active = activePrint{filename: update.Filename, startedAt: now}
		}
		if update.Progress > p.active.highestProgress {
			p.active.highestProgress = update.Progress
		}
	}

	status, final := statusToPrintStatus(update.RawPrintState)
	if !final || p.active.filename == "" {
		return printer, nil
	}

	completion := 100
	if status != domain.PrintStatusSuccess {
		progress := update.Progress
		if progress == 0 {
			progress = p.active.highestProgress
		}
		completion = int(progress * 100)
	}
	if completion <= 0 {
		completion = 100
	}

	filename := update.Filename
	if filename == "" {
		filename = p.active.filename
	}

	event := &domain.PrinterPrintEvent{
		PrinterID:         p.printerID,
		ExternalPrintID:   externalPrintID(p.printerID, filename, p.active.startedAt),
		GcodeFilename:     filename,
		Status:            status,
		CompletionPercent: completion,
		EstimatedLengthMM: update.FilamentUsedMM,
		PrintDuration:     update.PrintDuration,
		PrintedAt:         now,
	}
	p.active = activePrint{}
	return printer, event
}

func externalPrintID(printerID, filename string, startedAt time.Time) string {
	return fmt.Sprintf("%s:%s:%d", printerID, filename, startedAt.Unix())
}
