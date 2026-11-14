package service

import (
	"context"
	"fmt"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

type PrintService struct {
	prints   port.PrintJobRepository
	spools   port.SpoolRepository
	previews *PreviewImageStore
}

func NewPrintService(prints port.PrintJobRepository, spools port.SpoolRepository, previews *PreviewImageStore) *PrintService {
	return &PrintService{prints: prints, spools: spools, previews: previews}
}

func (s *PrintService) ChangeSpool(ctx context.Context, printJobID, spoolID string) error {
	job, err := s.prints.GetByID(ctx, printJobID)
	if err != nil {
		return fmt.Errorf("getting print job: %w", err)
	}
	var spool *domain.Spool
	if spoolID != "" {
		loaded, err := s.spools.GetByID(ctx, spoolID)
		if err != nil {
			return fmt.Errorf("getting spool: %w", err)
		}
		spool = loaded
	}
	if err := s.prints.UpdateSpool(ctx, printJobID, spoolID); err != nil {
		return fmt.Errorf("changing print job spool: %w", err)
	}
	if spool != nil && (job.ActualWeightG == 0 || job.SpoolID != spoolID) && job.EstimatedLengthMM > 0 {
		usage := job.Usage().WithSpool(*spool, job.Status)
		if err := savePrintUsage(ctx, s.prints, printJobID, usage); err != nil {
			return err
		}
	}
	return nil
}

func (s *PrintService) List(ctx context.Context, filter domain.PrintJobFilter) ([]domain.PrintJob, error) {
	jobs, err := s.prints.List(ctx, filter)
	if err != nil {
		return jobs, err
	}
	if err := preparePrintPreviews(ctx, s.previews, s.prints, jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *PrintService) GetStats(ctx context.Context, filter domain.StatsFilter) (*domain.PrintStats, error) {
	return s.prints.GetStats(ctx, filter)
}

func (s *PrintService) GetByID(ctx context.Context, id string) (*domain.PrintJob, error) {
	job, err := s.prints.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	jobs := []domain.PrintJob{*job}
	if err := preparePrintPreviews(ctx, s.previews, s.prints, jobs); err != nil {
		return nil, err
	}
	*job = jobs[0]
	return job, nil
}

func savePrintUsage(ctx context.Context, prints port.PrintJobRepository, id string, usage domain.PrintUsage) error {
	if err := prints.UpdateUsage(ctx, id, usage.EstimatedWeightG, usage.ActualWeightG, usage.LengthMM, usage.CompletionPercent, usage.Measured); err != nil {
		return fmt.Errorf("updating print job usage: %w", err)
	}
	return nil
}
