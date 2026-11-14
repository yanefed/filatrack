package dashboard

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/service"
)

const recentPrintLimit = 4

type dashboardSpool struct {
	domain.Spool
	RemainingG  float64
	NeedsDrying bool
}

type dashboardData struct {
	stats       *domain.PrintStats
	spools      []dashboardSpool
	printers    []domain.Printer
	recent      []domain.PrintJob
	statsErr    error
	spoolsErr   error
	printersErr error
	recentErr   error
}

func loadDashboardData(ctx context.Context, spoolSvc *service.SpoolService, printSvc *service.PrintService, printerSvc *service.PrinterService) dashboardData {
	data := dashboardData{}
	data.stats, data.statsErr = printSvc.GetStats(ctx, domain.StatsFilter{})

	notArchived := false
	summaries, err := spoolSvc.ListSummaries(ctx, domain.SpoolFilter{Archived: &notArchived})
	data.spoolsErr = err
	for _, summary := range summaries {
		if summary.DryingErr != nil {
			data.spoolsErr = errors.Join(data.spoolsErr, fmt.Errorf("checking drying for spool %s: %w", summary.Spool.ID, summary.DryingErr))
		}
		data.spools = append(data.spools, dashboardSpool{Spool: summary.Spool, RemainingG: summary.RemainingG, NeedsDrying: summary.Drying.NeedsDrying})
	}

	data.printers, data.printersErr = printerSvc.List(ctx)
	data.recent, data.recentErr = printSvc.List(ctx, domain.PrintJobFilter{Limit: recentPrintLimit})
	return data
}

func (data dashboardData) activePrinters() []domain.Printer {
	active := make([]domain.Printer, 0)
	for _, printer := range data.printers {
		if printer.State == domain.PrinterStatePrinting || printer.State == domain.PrinterStatePaused {
			active = append(active, printer)
		}
	}
	return active
}

func (data dashboardData) dryingWarnings() []dashboardSpool {
	warnings := make([]dashboardSpool, 0)
	for _, spool := range data.spools {
		if spool.NeedsDrying {
			warnings = append(warnings, spool)
		}
	}
	return warnings
}

func (data dashboardData) totalRemainingG() float64 {
	total := 0.0
	for _, spool := range data.spools {
		total += spool.RemainingG
	}
	return total
}
