package domain

import (
	"time"
)

// PrintHistoryMatch identifies either a remote job or its live-monitor duplicate.
type PrintHistoryMatch struct {
	ExternalPrintID string
	Filename        string
	EndedAt         time.Time
}

type PrintJobFilter struct {
	// Non-nil restricts results to the supplied history page; empty matches nothing.
	HistoryMatches []PrintHistoryMatch
	SpoolID        *string
	PrinterID      *string
	Status         *PrintStatus
	From           *time.Time
	To             *time.Time
	Limit          int
}

type StatsFilter struct {
	From *time.Time
	To   *time.Time
}

type PrintStats struct {
	TotalPrints    int
	SuccessCount   int
	FailedCount    int
	CancelledCount int
	TotalWeightG   float64
	AvgWeightG     float64
}
