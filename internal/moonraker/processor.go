package moonraker

import (
	"fmt"
	"time"

	"awesomeProject/internal/domain"
)

type Processor struct {
	printerID      string
	active         activePrint
	state          domain.PrinterState
	rawPrintState  string
	filename       string
	progress       float64
	filamentUsedMM float64
	printDuration  time.Duration
}

type activePrint struct {
	filename        string
	startedAt       time.Time
	highestProgress float64
}

func NewProcessor(printerID string) *Processor {
	return &Processor{printerID: printerID, state: domain.PrinterStateUnknown}
}

func (p *Processor) Apply(update StatusUpdate, now time.Time) (domain.Printer, *domain.PrinterPrintEvent) {
	if !update.HasState && update.RawPrintState != "" {
		update.HasState = true
		update.State = mapPrintState(update.RawPrintState)
	}
	if !update.HasFilename && update.Filename != "" {
		update.HasFilename = true
	}
	if !update.HasProgress && update.Progress != 0 {
		update.HasProgress = true
	}

	if update.HasState {
		p.state = update.State
		p.rawPrintState = update.RawPrintState
	}
	if update.HasFilename {
		p.filename = update.Filename
	}
	if update.HasProgress {
		p.progress = update.Progress
	}
	if update.FilamentUsedMM > 0 {
		p.filamentUsedMM = update.FilamentUsedMM
	}
	if update.PrintDuration > 0 {
		p.printDuration = update.PrintDuration
	}

	printer := domain.Printer{
		ID:          p.printerID,
		State:       p.state,
		CurrentFile: p.filename,
		Progress:    p.progress,
		LastSeenAt:  now,
	}

	if p.filename != "" && (p.rawPrintState == "printing" || p.rawPrintState == "paused") {
		if p.active.filename != p.filename {
			p.active = activePrint{filename: p.filename, startedAt: now}
		}
		if p.progress > p.active.highestProgress {
			p.active.highestProgress = p.progress
		}
	}

	status, final := statusToPrintStatus(p.rawPrintState)
	if !final || p.active.filename == "" {
		return printer, nil
	}

	completion := 100
	if status != domain.PrintStatusSuccess {
		progress := p.progress
		if progress == 0 {
			progress = p.active.highestProgress
		}
		completion = int(progress * 100)
	}
	if completion <= 0 {
		completion = 100
	}

	filename := p.filename
	if filename == "" {
		filename = p.active.filename
	}

	event := &domain.PrinterPrintEvent{
		PrinterID:         p.printerID,
		ExternalPrintID:   externalPrintID(p.printerID, filename, p.active.startedAt),
		GcodeFilename:     filename,
		Status:            status,
		CompletionPercent: completion,
		EstimatedLengthMM: p.filamentUsedMM,
		PrintDuration:     p.printDuration,
		PrintedAt:         now,
	}
	p.active = activePrint{}
	p.filamentUsedMM = 0
	p.printDuration = 0
	return printer, event
}

func externalPrintID(printerID, filename string, startedAt time.Time) string {
	return fmt.Sprintf("%s:%s:%d", printerID, filename, startedAt.Unix())
}
