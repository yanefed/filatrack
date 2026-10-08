package domain

import (
	"fmt"
	"strings"
	"time"
)

func makePrintFingerprint(filename string, printedAt time.Time) string {
	if filename == "" || printedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s:%d", filename, printedAt.Truncate(time.Minute).Unix())
}

func (p *PrintJob) Fingerprint() string {
	// Only old live records used a client-generated ID. Canonical remote IDs
	// must never match by filename/minute: distinct short prints may share both.
	if !strings.HasPrefix(p.ExternalPrintID, p.PrinterID+":"+p.GcodeFilename+":") {
		return ""
	}
	return makePrintFingerprint(p.GcodeFilename, p.PrintedAt)
}

func (j *PrinterHistoryJob) Fingerprint() string {
	return makePrintFingerprint(j.GcodeFilename, j.EndedAt)
}
