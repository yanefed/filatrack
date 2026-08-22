package dashboard

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
	"awesomeProject/internal/service"
	"context"
)

const recentPrintLimit = 4

type dashboardSpool struct {
	domain.Spool
	RemainingG  float64
	NeedsDrying bool
}

type dashboardData struct {
	stats       *repository.PrintStats
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
	data.stats, data.statsErr = printSvc.GetStats(ctx, repository.StatsFilter{})

	notArchived := false
	spools, err := spoolSvc.List(ctx, repository.SpoolFilter{Archived: &notArchived})
	data.spoolsErr = err
	if err == nil {
		data.spools = make([]dashboardSpool, 0, len(spools))
		for i := range spools {
			_, remaining, remainingErr := spoolSvc.GetWithRemaining(ctx, spools[i].ID)
			if remainingErr != nil {
				remaining = spools[i].TotalWeightG
			}
			needsDrying, _ := spoolSvc.NeedsDrying(ctx, &spools[i])
			data.spools = append(data.spools, dashboardSpool{Spool: spools[i], RemainingG: remaining, NeedsDrying: needsDrying})
		}
	}

	data.printers, data.printersErr = printerSvc.List(ctx)
	data.recent, data.recentErr = printSvc.List(ctx, repository.PrintJobFilter{Limit: recentPrintLimit})
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
