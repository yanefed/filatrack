package service

import (
	"context"
	"fmt"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/gcode"
	"awesomeProject/internal/repository"
)

type PrintService struct {
	prints repository.PrintJobRepository
	spools repository.SpoolRepository
}

func NewPrintService(prints repository.PrintJobRepository, spools repository.SpoolRepository) *PrintService {
	return &PrintService{prints: prints, spools: spools}
}

func (s *PrintService) CreateFromGcode(ctx context.Context, spoolID string, meta *domain.GcodeMetadata, status domain.PrintStatus, completionPercent int) (*domain.PrintJob, error) {
	spool, err := s.spools.GetByID(ctx, spoolID)
	if err != nil {
		return nil, fmt.Errorf("getting spool: %w", err)
	}

	weightG := meta.FilamentWeightG
	if weightG == 0 && meta.FilamentLengthMM > 0 {
		weightG = gcode.WeightFromLength(meta.FilamentLengthMM, spool.DiameterMM, spool.DensityGCM3)
	}

	if status == domain.PrintStatusSuccess {
		completionPercent = 100
	}

	job := &domain.PrintJob{
		SpoolID:           spoolID,
		GcodeFilename:     meta.Filename,
		ModelName:         meta.ModelName,
		Status:            status,
		CompletionPercent: completionPercent,
		EstimatedWeightG:  weightG,
		ActualWeightG:     weightG * float64(completionPercent) / 100,
		EstimatedLengthMM: meta.FilamentLengthMM,
		PrintDuration:     meta.PrintDuration,
		NozzleTempC:       meta.NozzleTempC,
		BedTempC:          meta.BedTempC,
		Slicer:            meta.Slicer,
		PrintedAt:         time.Now(),
		CreatedAt:         time.Now(),
	}

	if err := s.prints.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating print job: %w", err)
	}
	return job, nil
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
		actualWeight := gcode.WeightFromLength(job.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
		estimatedWeight := job.EstimatedWeightG
		if estimatedWeight == 0 {
			estimatedWeight = actualWeight
		}
		completion := job.CompletionPercent
		if completion <= 0 && estimatedWeight > 0 {
			completion = int(actualWeight / estimatedWeight * 100)
			if completion < 0 {
				completion = 0
			}
			if completion > 100 {
				completion = 100
			}
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
