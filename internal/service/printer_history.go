package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

type printerHistory struct {
	historySyncMu sync.Mutex
	printers      port.PrinterRepository
	spools        port.SpoolRepository
	prints        port.PrintJobRepository
	previews      *PreviewImageStore
	settings      port.SettingsRepository
	jobClient     port.PrinterJobClient
}

func (s *printerHistory) RecordPrinterPrint(ctx context.Context, event domain.PrinterPrintEvent) (*domain.PrintJob, error) {
	if event.ExternalPrintID == "" {
		return nil, fmt.Errorf("printer print event has no external job ID")
	}
	printer, err := s.printers.GetByID(ctx, event.PrinterID)
	if err != nil {
		return nil, fmt.Errorf("getting printer: %w", err)
	}
	if printer.ActiveSpoolID == "" {
		return nil, fmt.Errorf("printer %s has no active spool assigned", printer.Name)
	}
	spool, err := s.spools.GetByID(ctx, printer.ActiveSpoolID)
	if err != nil {
		return nil, fmt.Errorf("getting active spool: %w", err)
	}

	job := domain.CreatePrinterJobFromPrinterEvent(*printer, event, *spool)
	if s.previews != nil && len(job.PreviewImageData) > 0 {
		hash, path, err := s.previews.Store(ctx, job.PreviewImageData)
		if err != nil {
			return nil, fmt.Errorf("storing print preview image: %w", err)
		}
		job.PreviewImageHash = hash
		job.PreviewImagePath = path
	}
	if err := s.prints.Create(ctx, job); err != nil {
		if errors.Is(err, domain.ErrPrintJobAlreadyExists) {
			stored, lookupErr := s.findRecordedPrinterJob(ctx, job.PrinterID, job.ExternalPrintID)
			if lookupErr != nil {
				return nil, lookupErr
			}
			// A background import may have won the insert before the live
			// event could associate the spool that was actually in use.
			if stored.SpoolID == "" {
				if err := s.prints.UpdateSpool(ctx, stored.ID, job.SpoolID); err != nil {
					return nil, fmt.Errorf("assigning recorded print spool: %w", err)
				}
				if err := savePrintUsage(ctx, s.prints, stored.ID, job.Usage()); err != nil {
					return nil, fmt.Errorf("updating recorded print usage: %w", err)
				}
				stored.SpoolID = job.SpoolID
				job.Usage().Apply(stored)
			}
			return stored, nil
		}
		return nil, fmt.Errorf("creating printer print job: %w", err)
	}
	return job, nil
}

func (s *printerHistory) ListCombinedHistory(ctx context.Context, printerID string, limit int) ([]domain.PrinterHistoryJob, error) {
	if s.jobClient == nil {
		return nil, fmt.Errorf("printer job client is not configured")
	}
	printer, err := s.printers.GetByID(ctx, printerID)
	if err != nil {
		return nil, fmt.Errorf("getting printer: %w", err)
	}
	remoteJobs, err := s.jobClient.ListHistory(ctx, *printer, limit)
	if err != nil {
		return nil, err
	}
	localJobs, err := s.prints.List(ctx, domain.PrintJobFilter{PrinterID: &printerID})
	if err != nil {
		return nil, fmt.Errorf("listing local printer prints: %w", err)
	}
	if err := preparePrintPreviews(ctx, s.previews, s.prints, localJobs); err != nil {
		return nil, err
	}
	localByExternalID := make(map[string]domain.PrintJob, len(localJobs))
	for _, job := range localJobs {
		if job.ExternalPrintID != "" {
			localByExternalID[job.ExternalPrintID] = job
		}
	}
	for i := range remoteJobs {
		if local, ok := localByExternalID[remoteJobs[i].ExternalPrintID]; ok {
			remoteJobs[i].LocalPrintJobID = local.ID
			remoteJobs[i].LocalSpoolID = local.SpoolID
			remoteJobs[i].PreviewImagePath = local.PreviewImagePath
			remoteJobs[i].PreviewImageHash = local.PreviewImageHash
		}
	}
	for i := range remoteJobs {
		path, err := s.previews.Resolve(ctx, remoteJobs[i].PreviewImageHash, remoteJobs[i].PreviewImagePath)
		if err != nil {
			return nil, fmt.Errorf("loading history preview image: %w", err)
		}
		remoteJobs[i].PreviewImagePath = path
	}
	return remoteJobs, nil
}

func (s *printerHistory) SyncPrinterHistory(ctx context.Context, printerID string, limit int) (int, error) {
	// Manual and background imports must not race their checkpoints or inserts.
	s.historySyncMu.Lock()
	defer s.historySyncMu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s.jobClient == nil {
		return 0, fmt.Errorf("printer job client is not configured")
	}
	printer, err := s.printers.GetByID(ctx, printerID)
	if err != nil {
		return 0, fmt.Errorf("getting printer: %w", err)
	}
	var activeSpool *domain.Spool
	if printer.ActiveSpoolID != "" {
		spool, err := s.spools.GetByID(ctx, printer.ActiveSpoolID)
		if err != nil {
			return 0, fmt.Errorf("getting active spool: %w", err)
		}
		activeSpool = spool
	}
	latest, err := s.prints.List(ctx, domain.PrintJobFilter{PrinterID: &printerID, Limit: 1})
	if err != nil {
		return 0, fmt.Errorf("getting newest local printer print: %w", err)
	}
	newestLocal := newestPrintTime(latest)

	importJobs := func(jobs []domain.PrinterHistoryJob) (int, error) {
		return s.importHistoryPage(ctx, *printer, activeSpool, newestLocal, jobs)
	}

	if limit <= 0 {
		if pageClient, ok := s.jobClient.(port.PrinterHistoryPageClient); ok {
			return s.syncHistoryPages(ctx, *printer, pageClient, importJobs)
		}
	}

	remoteJobs, err := s.jobClient.ListHistory(ctx, *printer, limit)
	if err != nil {
		return 0, err
	}
	return importJobs(remoteJobs)
}

func (s *printerHistory) AssignHistoryJobSpool(ctx context.Context, printerID, externalPrintID, spoolID string) (*domain.PrintJob, error) {
	if spoolID == "" {
		return nil, fmt.Errorf("spool is required")
	}
	spool, err := s.spools.GetByID(ctx, spoolID)
	if err != nil {
		return nil, fmt.Errorf("getting spool: %w", err)
	}
	history, err := s.ListCombinedHistory(ctx, printerID, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range history {
		if item.ExternalPrintID != externalPrintID {
			continue
		}
		if item.LocalPrintJobID != "" {
			return s.assignRecordedJobSpool(ctx, item.LocalPrintJobID, spool)
		}
		job, err := s.createPrintFromHistory(ctx, item, spool)
		if errors.Is(err, domain.ErrPrintJobAlreadyExists) {
			stored, lookupErr := s.findRecordedPrinterJob(ctx, printerID, externalPrintID)
			if lookupErr != nil {
				return nil, lookupErr
			}
			return s.assignRecordedJobSpool(ctx, stored.ID, spool)
		}
		return job, err
	}
	return nil, fmt.Errorf("printer history job not found: %s", externalPrintID)
}

func (s *printerHistory) updateExistingHistoryUsage(ctx context.Context, local domain.PrintJob, item domain.PrinterHistoryJob) error {

	var spool *domain.Spool
	if local.SpoolID != "" {
		loaded, err := s.spools.GetByID(ctx, local.SpoolID)
		if err != nil {
			return fmt.Errorf("getting print job spool: %w", err)
		}
		spool = loaded
	}
	usage := local.Usage().WithHistory(item, spool, local.Status)
	if usage == local.Usage() {
		return nil
	}
	return savePrintUsage(ctx, s.prints, local.ID, usage)
}

func (s *printerHistory) createPrintFromHistory(ctx context.Context, item domain.PrinterHistoryJob, spool *domain.Spool) (*domain.PrintJob, error) {
	status, ok := item.PrintStatus()
	if !ok {
		return nil, fmt.Errorf("only finished printer jobs can be assigned to a spool")
	}

	job := domain.CreatePrinterJobFromPrinterHistoryItem(item, spool, status)

	if err := s.prints.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating print from printer history: %w", err)
	}
	return job, nil
}

func (s *printerHistory) enrichHistoryJob(ctx context.Context, printer domain.Printer, job *domain.PrinterHistoryJob) {
	if s.jobClient == nil || job == nil || job.GcodeFilename == "" {
		return
	}
	metadata, err := s.jobClient.FetchMetadata(ctx, printer, job.GcodeFilename)
	if err != nil || metadata == nil {
		return
	}
	if metadata.FilamentWeightG > 0 {
		job.EstimatedWeightG = metadata.FilamentWeightG
	}
	if metadata.PrintDuration > 0 && job.PrintDuration == 0 {
		job.PrintDuration = metadata.PrintDuration
	}
	if metadata.NozzleTempC > 0 {
		job.NozzleTempC = metadata.NozzleTempC
	}
	if metadata.BedTempC > 0 {
		job.BedTempC = metadata.BedTempC
	}
	if metadata.Slicer != "" {
		job.Slicer = metadata.Slicer
	}
	if metadata.PreviewImagePath != "" {
		job.PreviewImagePath = metadata.PreviewImagePath
	}
	if metadata.PreviewImageHash != "" {
		job.PreviewImageHash = metadata.PreviewImageHash
	}
	if len(metadata.PreviewImageData) > 0 {
		job.PreviewImageData = metadata.PreviewImageData
	}
}

func newestPrintTime(jobs []domain.PrintJob) time.Time {
	var newest time.Time
	for _, job := range jobs {
		if job.PrintedAt.After(newest) {
			newest = job.PrintedAt
		}
	}
	return newest
}

func (s *printerHistory) findRecordedPrinterJob(ctx context.Context, printerID, externalID string) (*domain.PrintJob, error) {
	jobs, err := s.prints.List(ctx, domain.PrintJobFilter{PrinterID: &printerID,
		HistoryMatches: []domain.PrintHistoryMatch{{ExternalPrintID: externalID}}})
	if err != nil {
		return nil, fmt.Errorf("getting recorded printer job: %w", err)
	}
	if len(jobs) != 1 {
		return nil, fmt.Errorf("recorded printer job not found: %s", externalID)
	}
	return &jobs[0], nil
}

func (s *printerHistory) assignRecordedJobSpool(ctx context.Context, jobID string, spool *domain.Spool) (*domain.PrintJob, error) {
	if err := s.prints.UpdateSpool(ctx, jobID, spool.ID); err != nil {
		return nil, fmt.Errorf("changing print job spool: %w", err)
	}
	job, err := s.prints.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.EstimatedLengthMM > 0 {
		usage := job.Usage().WithSpool(*spool, job.Status)
		// A history length is measured consumption, even when its weight is derived.
		usage.Measured = true
		if err := savePrintUsage(ctx, s.prints, job.ID, usage); err != nil {
			return nil, err
		}
		usage.Apply(job)
	}
	job.PreviewImagePath, err = s.previews.Resolve(ctx, job.PreviewImageHash, job.PreviewImagePath)
	if err != nil {
		return nil, fmt.Errorf("loading print preview image: %w", err)
	}
	return job, nil
}

func (s *printerHistory) importHistoryPage(ctx context.Context, printer domain.Printer, activeSpool *domain.Spool, newestLocal time.Time, remoteJobs []domain.PrinterHistoryJob) (int, error) {
	printerID := printer.ID

	matches := make([]domain.PrintHistoryMatch, 0, len(remoteJobs))
	for _, job := range remoteJobs {
		matches = append(matches, domain.PrintHistoryMatch{ExternalPrintID: job.ExternalPrintID, Filename: job.GcodeFilename, EndedAt: job.EndedAt})
	}
	localJobs, err := s.prints.List(ctx, domain.PrintJobFilter{PrinterID: &printerID, HistoryMatches: matches})
	if err != nil {
		return 0, fmt.Errorf("matching local printer prints: %w", err)
	}
	existingJobs := make(map[string]domain.PrintJob, len(localJobs))
	existingFingerprints := make(map[string]struct{}, len(localJobs))
	for _, job := range localJobs {
		if job.ExternalPrintID != "" {
			existingJobs[job.ExternalPrintID] = job
		}
		if fingerprint := job.Fingerprint(); fingerprint != "" {
			existingFingerprints[fingerprint] = struct{}{}
		}
	}
	s.enrichHistoryPage(ctx, printer, remoteJobs, existingJobs, existingFingerprints)

	created := 0
	for _, job := range remoteJobs {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		if job.ExternalPrintID == "" {
			continue
		}
		if local, ok := existingJobs[job.ExternalPrintID]; ok {
			job.PrinterID = printerID
			local, err = s.recoverHistoryPreview(ctx, local, job)
			if err != nil {
				return created, err
			}
			existingJobs[job.ExternalPrintID] = local

			if err := s.updateExistingHistoryUsage(ctx, local, job); err != nil {
				return created, err
			}
			continue
		}
		if fingerprint := job.Fingerprint(); fingerprint != "" {
			if _, ok := existingFingerprints[fingerprint]; ok {
				continue
			}
		}
		if _, ok := job.PrintStatus(); !ok {
			continue
		}
		job.PrinterID = printerID
		if len(job.PreviewImageData) > 0 && s.previews != nil {
			hash, path, storeErr := s.previews.Store(ctx, job.PreviewImageData)
			if storeErr != nil {
				return created, fmt.Errorf("storing print history preview image: %w", storeErr)
			}
			job.PreviewImageHash = hash
			job.PreviewImagePath = path
		}
		spoolForJob := activeSpool
		if newestLocal.IsZero() || !job.EndedAt.After(newestLocal) {
			spoolForJob = nil
		}
		local, err := s.createPrintFromHistory(ctx, job, spoolForJob)
		if errors.Is(err, domain.ErrPrintJobAlreadyExists) {
			continue
		}
		if err != nil {
			return created, err
		}
		existingJobs[job.ExternalPrintID] = *local
		created++
	}
	return created, nil
}

func (s *printerHistory) enrichHistoryPage(ctx context.Context, printer domain.Printer, remoteJobs []domain.PrinterHistoryJob, existingJobs map[string]domain.PrintJob, existingFingerprints map[string]struct{}) {
	enrichIndexes := make([]int, 0, len(remoteJobs))
	for i := range remoteJobs {
		job := remoteJobs[i]
		if job.ExternalPrintID == "" {
			continue
		}
		if local, ok := existingJobs[job.ExternalPrintID]; ok {
			// Actual consumed length comes directly from the history response.
			if local.PreviewImageHash == "" || local.EstimatedWeightG <= 0 && job.EstimatedWeightG <= 0 {
				enrichIndexes = append(enrichIndexes, i)
			}
			continue
		}
		if fingerprint := job.Fingerprint(); fingerprint != "" {
			if _, ok := existingFingerprints[fingerprint]; ok {
				continue
			}
		}
		if _, ok := job.PrintStatus(); ok {
			enrichIndexes = append(enrichIndexes, i)
		}
	}
	parallelFor(ctx, len(enrichIndexes), metadataSyncConcurrency, func(i int) {
		index := enrichIndexes[i]
		remoteJobs[index].PrinterID = printer.ID
		s.enrichHistoryJob(ctx, printer, &remoteJobs[index])
	})

}

func (s *printerHistory) recoverHistoryPreview(ctx context.Context, local domain.PrintJob, remote domain.PrinterHistoryJob) (domain.PrintJob, error) {
	if local.PreviewImageHash != "" || len(remote.PreviewImageData) == 0 || s.previews == nil {
		return local, nil
	}
	hash, path, err := s.previews.Store(ctx, remote.PreviewImageData)
	if err != nil {
		return local, fmt.Errorf("storing recovered print preview image: %w", err)
	}
	if hash == "" {
		return local, nil
	}
	if err := s.prints.UpdatePreviewImageHash(ctx, local.ID, hash); err != nil {
		return local, fmt.Errorf("saving recovered print preview: %w", err)
	}
	local.PreviewImageHash, local.PreviewImagePath = hash, path
	return local, nil
}
