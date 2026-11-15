package domain

import (
	"errors"
	"time"
)

var ErrSpoolInUse = errors.New("spool is used by an active or paused print")
var ErrSpoolAssigned = errors.New("confirm detaching the spool from its printers")
var ErrArchiveChanged = errors.New("spool archive changed since this operation")

type SpoolArchive struct {
	SpoolID    string
	ArchivedAt time.Time
	Printers   []Printer
}

type SpoolRestoreResult struct{ SkippedPrinterIDs []string }
