package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
	"context"
	"fmt"
)

type PrintService struct {
	prints repository.PrintJobRepository
	spools repository.SpoolRepository
}

func NewPrintService(prints repository.PrintJobRepository, spools repository.SpoolRepository) *PrintService {
	return &PrintService{prints: prints, spools: spools}
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
	if spool != nil && job.ActualWeightG == 0 && job.EstimatedLengthMM > 0 {
		actualWeight := domain.WeightFromLength(job.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
		estimatedWeight := job.EstimatedWeightG
		if estimatedWeight == 0 {
			estimatedWeight = actualWeight
		}
		completion := job.CompletionPercent
		if completion <= 0 && estimatedWeight > 0 {
			completion = min(max(int(actualWeight/estimatedWeight*100), 0), 100)
		}
		if err := s.prints.UpdateUsage(ctx, printJobID, estimatedWeight, actualWeight, job.EstimatedLengthMM, completion); err != nil {
			return fmt.Errorf("updating print job usage: %w", err)
		}
	}
	return nil
}

func (s *PrintService) List(ctx context.Context, filter repository.PrintJobFilter) ([]domain.PrintJob, error) {
	return s.prints.List(ctx, filter)
}

func (s *PrintService) GetStats(ctx context.Context, filter repository.StatsFilter) (*repository.PrintStats, error) {
	return s.prints.GetStats(ctx, filter)
}
