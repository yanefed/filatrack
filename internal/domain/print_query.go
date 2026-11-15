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

type PrintJobCursor struct {
	PrintedAt time.Time
	ID        string
}

type PrintJobFilter struct {
	Search string
	// Before excludes the cursor; Through includes it and all newer jobs.
	Before  *PrintJobCursor
	Through *PrintJobCursor
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
	TotalCost          float64
	CostCurrency       string
	UnpricedCount      int
	EstimatedCostCount int
	TotalPrints        int
	SuccessCount       int
	FailedCount        int
	CancelledCount     int
	TotalWeightG       float64
	AvgWeightG         float64
}
