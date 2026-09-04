package domain

import (
	"fmt"
	"time"
)

func makePrintFingerprint(filename string, printedAt time.Time) string {
	if filename == "" || printedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s:%d", filename, printedAt.Truncate(time.Minute).Unix())
}

func (j *PrintJob) Fingerprint() string {
	return makePrintFingerprint(j.GcodeFilename, j.PrintedAt)
}

func (j *PrinterHistoryJob) Fingerprint() string {
	return makePrintFingerprint(j.GcodeFilename, j.EndedAt)
}
