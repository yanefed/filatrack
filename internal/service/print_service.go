package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/port"
	"context"
	"fmt"
	"os"
)

type PrintService struct {
	prints   port.PrintJobRepository
	spools   port.SpoolRepository
	previews *PreviewImageStore
}

func NewPrintService(prints port.PrintJobRepository, spools port.SpoolRepository) *PrintService {
	return &PrintService{prints: prints, spools: spools}
}

func (s *PrintService) SetPreviewImageStore(previews *PreviewImageStore) {
	s.previews = previews
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
		actualWeight := domain.WeightFromLength(job.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
		estimatedWeight := job.EstimatedWeightG
		if estimatedWeight == 0 && job.Status == domain.PrintStatusSuccess {
			estimatedWeight = actualWeight
		}
		completion := job.CompletionPercent
		if completion <= 0 && estimatedWeight > 0 {
			completion = min(max(int(actualWeight/estimatedWeight*100), 0), 100)
		}
		if err := s.prints.UpdateUsage(ctx, printJobID, estimatedWeight, actualWeight, job.EstimatedLengthMM, completion, job.UsageMeasured); err != nil {
			return fmt.Errorf("updating print job usage: %w", err)
		}
	}
	return nil
}

func (s *PrintService) List(ctx context.Context, filter domain.PrintJobFilter) ([]domain.PrintJob, error) {
	jobs, err := s.prints.List(ctx, filter)
	if err != nil {
		return jobs, err
	}
	if err := s.migrateLegacyPreviews(ctx, jobs); err != nil {
		return nil, err
	}
	if err := s.previews.HydratePrintJobs(ctx, jobs); err != nil {
		return nil, fmt.Errorf("loading print preview images: %w", err)
	}
	return jobs, nil
}

func (s *PrintService) migrateLegacyPreviews(ctx context.Context, jobs []domain.PrintJob) error {
	return migrateLegacyPrintPreviews(ctx, s.previews, s.prints, jobs)
}

func migrateLegacyPrintPreviews(ctx context.Context, previews *PreviewImageStore, prints port.PrintJobRepository, jobs []domain.PrintJob) error {
	if previews == nil {
		return nil
	}
	updater, ok := prints.(port.PrintJobPreviewUpdater)
	if !ok {
		return nil
	}
	for i := range jobs {
		if jobs[i].PreviewImageHash != "" || jobs[i].PreviewImagePath == "" {
			continue
		}
		hash, path, err := previews.ImportLegacy(ctx, jobs[i].PreviewImagePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("importing legacy print preview image: %w", err)
		}
		if hash == "" {
			continue
		}
		if err := updater.UpdatePreviewImageHash(ctx, jobs[i].ID, hash); err != nil {
			return fmt.Errorf("saving shared print preview reference: %w", err)
		}
		jobs[i].PreviewImageHash = hash
		jobs[i].PreviewImagePath = path
	}
	return nil
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
	if err := s.migrateLegacyPreviews(ctx, jobs); err != nil {
		return nil, err
	}
	*job = jobs[0]
	path, err := s.previews.Resolve(ctx, job.PreviewImageHash, job.PreviewImagePath)
	if err != nil {
		return nil, fmt.Errorf("loading print preview image: %w", err)
	}
	job.PreviewImagePath = path
	return job, nil
}
