package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/gcode"
	"github.com/yanefed/filatrack/internal/port"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metadataSyncConcurrency = 4

type PrinterService struct {
	historySyncMu sync.Mutex
	transferMu    sync.Mutex
	logger        *slog.Logger
	printers      port.PrinterRepository
	spools        port.SpoolRepository
	prints        port.PrintJobRepository
	models        port.PrinterModelRepository
	previews      *PreviewImageStore
	settings      port.SettingsRepository
	checker       port.ConnectionChecker
	jobClient     port.PrinterJobClient
}

func NewPrinterService(printers port.PrinterRepository, spools port.SpoolRepository, prints port.PrintJobRepository, settings port.SettingsRepository) *PrinterService {
	return NewPrinterServiceWithLogger(printers, spools, prints, settings, slog.Default())
}

func NewPrinterServiceWithLogger(
	printers port.PrinterRepository,
	spools port.SpoolRepository,
	prints port.PrintJobRepository,
	settings port.SettingsRepository,
	logger *slog.Logger,
) *PrinterService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PrinterService{
		logger:   logger.With("service", "printer"),
		printers: printers,
		spools:   spools,
		prints:   prints,
		settings: settings,
	}
}

func (s *PrinterService) SetConnectionChecker(checker port.ConnectionChecker) {
	s.checker = checker
}

func (s *PrinterService) SetJobClient(client port.PrinterJobClient) {
	s.jobClient = client
}

func (s *PrinterService) SetModelCache(models port.PrinterModelRepository) {
	s.models = models
}

func (s *PrinterService) SetPreviewImageStore(previews *PreviewImageStore) {
	s.previews = previews
}

func (s *PrinterService) CreateMoonrakerPrinter(ctx context.Context, printer *domain.Printer) error {
	if strings.TrimSpace(printer.Name) == "" {
		return fmt.Errorf("printer name is required")
	}
	if err := printer.ValidateMoonrakerURL(); err != nil {
		return err
	}
	now := time.Now()
	printer.Name = strings.TrimSpace(printer.Name)
	printer.MoonrakerURL = strings.TrimRight(strings.TrimSpace(printer.MoonrakerURL), "/")
	printer.Kind = domain.PrinterKindMoonraker
	printer.Enabled = true
	printer.State = domain.PrinterStateUnknown
	printer.CreatedAt = now
	printer.UpdatedAt = now
	return s.printers.Create(ctx, printer)
}

func (s *PrinterService) List(ctx context.Context) ([]domain.Printer, error) {
	return s.printers.List(ctx)
}

func (s *PrinterService) CheckMoonrakerConnection(ctx context.Context, printer *domain.Printer) error {
	if strings.TrimSpace(printer.Name) == "" {
		printer.Name = "Moonraker"
	}
	if err := printer.ValidateMoonrakerURL(); err != nil {
		return err
	}
	if s.checker == nil {
		return nil
	}
	candidate := *printer
	candidate.MoonrakerURL = strings.TrimRight(strings.TrimSpace(candidate.MoonrakerURL), "/")
	return s.checker.Check(ctx, candidate)
}

func (s *PrinterService) UpdateMoonrakerPrinter(ctx context.Context, printer *domain.Printer) error {
	if printer == nil {
		return fmt.Errorf("printer is required")
	}
	if strings.TrimSpace(printer.Name) == "" {
		return fmt.Errorf("printer name is required")
	}
	if err := printer.ValidateMoonrakerURL(); err != nil {
		return err
	}

	printer.Name = strings.TrimSpace(printer.Name)
	printer.MoonrakerURL = strings.TrimRight(strings.TrimSpace(printer.MoonrakerURL), "/")
	if err := s.CheckMoonrakerConnection(ctx, printer); err != nil {
		return fmt.Errorf("checking printer connection: %w", err)
	}
	return s.printers.Update(ctx, printer)
}

func (s *PrinterService) AssignSpool(ctx context.Context, printerID, spoolID string) error {
	if _, err := s.spools.GetByID(ctx, spoolID); err != nil {
		return fmt.Errorf("getting spool: %w", err)
	}
	if err := s.printers.AssignSpool(ctx, printerID, spoolID); err != nil {
		return err
	}
	return nil
}

func (s *PrinterService) ClearAssignedSpool(ctx context.Context, printerID string) error {
	return s.printers.AssignSpool(ctx, printerID, "")
}

func (s *PrinterService) UpdateStatus(ctx context.Context, printer *domain.Printer) error {
	if printer.State == "" {
		printer.State = domain.PrinterStateUnknown
	}
	if printer.Progress < 0 {
		printer.Progress = 0
	}
	if printer.Progress > 1 {
		printer.Progress = 1
	}
	if printer.LastSeenAt.IsZero() {
		printer.LastSeenAt = time.Now()
	}
	return s.printers.UpdateStatus(ctx, printer)
}

func (s *PrinterService) RecordPrinterPrint(ctx context.Context, event domain.PrinterPrintEvent) (*domain.PrintJob, error) {
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
				if err := s.prints.UpdateUsage(ctx, stored.ID, job.EstimatedWeightG, job.ActualWeightG, job.EstimatedLengthMM, job.CompletionPercent, job.UsageMeasured); err != nil {
					return nil, fmt.Errorf("updating recorded print usage: %w", err)
				}
				stored.SpoolID = job.SpoolID
				stored.EstimatedWeightG, stored.ActualWeightG = job.EstimatedWeightG, job.ActualWeightG
				stored.EstimatedLengthMM, stored.CompletionPercent, stored.UsageMeasured = job.EstimatedLengthMM, job.CompletionPercent, job.UsageMeasured
			}
			return stored, nil
		}
		return nil, fmt.Errorf("creating printer print job: %w", err)
	}
	return job, nil
}

func (s *PrinterService) ListCombinedHistory(ctx context.Context, printerID string, limit int) ([]domain.PrinterHistoryJob, error) {
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
	if err := migrateLegacyPrintPreviews(ctx, s.previews, s.prints, localJobs); err != nil {
		return nil, fmt.Errorf("migrating legacy print preview images: %w", err)
	}
	if err := s.previews.HydratePrintJobs(ctx, localJobs); err != nil {
		return nil, fmt.Errorf("loading local print preview images: %w", err)
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

func (s *PrinterService) ListModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
	if s.models != nil {
		models, err := s.models.List(ctx, &printerID)
		if err != nil {
			return nil, err
		}
		if err := s.previews.HydrateModels(ctx, models); err != nil {
			return nil, fmt.Errorf("loading printer model previews: %w", err)
		}
		return models, nil
	}
	return s.RefreshModels(ctx, printerID)
}

func (s *PrinterService) RefreshModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
	if s.jobClient == nil {
		return nil, fmt.Errorf("printer job client is not configured")
	}
	printer, err := s.printers.GetByID(ctx, printerID)
	if err != nil {
		return nil, fmt.Errorf("getting printer: %w", err)
	}
	if printer.State == domain.PrinterStateOffline {
		return nil, fmt.Errorf("printer %q is offline; models were not refreshed", printer.Name)
	}
	models, err := s.jobClient.ListModels(ctx, *printer)
	if err != nil {
		return nil, err
	}
	cachedByPath := make(map[string]domain.PrinterModelFile)
	if s.models != nil {
		cached, cacheErr := s.models.List(ctx, &printerID)
		if cacheErr == nil {
			for _, model := range cached {
				cachedByPath[model.Path] = model
			}
		} else {
			s.logger.Warn("loading cached printer models failed", "printer_id", printerID, "error", cacheErr)
		}
	}
	diameterMM, densityGCM3 := s.modelFilamentProperties(ctx, printer)
	var imageErr error
	var imageErrMu sync.Mutex
	parallelFor(ctx, len(models), metadataSyncConcurrency, func(i int) {
		if cached, ok := cachedByPath[models[i].Path]; ok {
			models[i].EstimatedWeightG = cached.EstimatedWeightG
			models[i].PrintDuration = cached.PrintDuration
			models[i].Slicer = cached.Slicer
			models[i].PreviewImagePath = cached.PreviewImagePath
			models[i].PreviewImageHash = cached.PreviewImageHash
		}
		metadata, metadataErr := s.jobClient.FetchMetadata(ctx, *printer, models[i].Path)
		if metadataErr != nil {
			s.logger.Debug("fetching printer model metadata failed", "printer_id", printerID, "filename", models[i].Path, "error", metadataErr)
			metadata = nil
		}
		if metadata == nil {
			metadata = &domain.GcodeMetadata{Filename: models[i].Path}
		}
		if metadata.FilamentWeightG <= 0 && metadata.FilamentLengthMM <= 0 && models[i].EstimatedWeightG <= 0 {
			localMetadata, downloadErr := s.downloadModelMetadata(ctx, *printer, models[i].Path)
			if downloadErr == nil {
				metadata.MergeMissingGcodeMetadata(localMetadata)
			} else {
				s.logger.Debug("downloading printer model metadata failed", "printer_id", printerID, "filename", models[i].Path, "error", downloadErr)
			}
		}
		weightG := metadata.FilamentWeightG
		if weightG <= 0 && metadata.FilamentLengthMM > 0 {
			weightG = domain.WeightFromLength(metadata.FilamentLengthMM, diameterMM, densityGCM3)
		}
		if weightG > 0 {
			models[i].EstimatedWeightG = weightG
		}
		if metadata.PrintDuration > 0 {
			models[i].PrintDuration = metadata.PrintDuration
		}
		if metadata.Slicer != "" {
			models[i].Slicer = metadata.Slicer
		}
		if len(metadata.PreviewImageData) > 0 && s.previews != nil {
			hash, path, storeErr := s.previews.Store(ctx, metadata.PreviewImageData)
			if storeErr != nil {
				imageErrMu.Lock()
				if imageErr == nil {
					imageErr = storeErr
				}
				imageErrMu.Unlock()
				return
			}
			models[i].PreviewImageHash = hash
			models[i].PreviewImagePath = path
		} else if metadata.PreviewImagePath != "" {
			models[i].PreviewImagePath = metadata.PreviewImagePath
		}
	})
	if imageErr != nil {
		return nil, fmt.Errorf("storing printer model preview: %w", imageErr)
	}
	if s.models != nil {
		if err := s.models.SaveForPrinter(ctx, printerID, models); err != nil {
			return nil, err
		}
		models, err = s.models.List(ctx, &printerID)
		if err != nil {
			return nil, err
		}
	}
	if err := s.previews.HydrateModels(ctx, models); err != nil {
		return nil, fmt.Errorf("loading printer model previews: %w", err)
	}
	return models, nil
}

// TODO refactor
func (s *PrinterService) modelFilamentProperties(ctx context.Context, printer *domain.Printer) (float64, float64) {
	diameterMM := domain.DefaultDiameter
	densityGCM3 := domain.DefaultDensity
	if printer.ActiveSpoolID == "" {
		return diameterMM, densityGCM3
	}

	spool, err := s.spools.GetByID(ctx, printer.ActiveSpoolID)
	if err != nil || spool == nil {
		return diameterMM, densityGCM3
	}
	if spool.DiameterMM > 0 {
		diameterMM = spool.DiameterMM
	}
	if spool.DensityGCM3 > 0 {
		densityGCM3 = spool.DensityGCM3
	}
	return diameterMM, densityGCM3
}

func (s *PrinterService) downloadModelMetadata(ctx context.Context, printer domain.Printer, filename string) (*domain.GcodeMetadata, error) {
	tmpDir, err := os.MkdirTemp("", "filatrack-model-metadata-*")
	if err != nil {
		return nil, fmt.Errorf("creating model metadata cache: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			s.logger.Warn("removing model metadata cache failed", "path", tmpDir, "error", removeErr)
		}
	}()

	localPath := filepath.Join(tmpDir, filepath.Base(filename))
	if err := s.jobClient.DownloadFile(ctx, printer, filename, localPath); err != nil {
		return nil, err
	}
	return gcode.ParseFile(localPath)
}

func (s *PrinterService) ListAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
	if s.models != nil {
		models, err := s.models.List(ctx, nil)
		if err != nil {
			return nil, err
		}
		if err := s.previews.HydrateModels(ctx, models); err != nil {
			return nil, fmt.Errorf("loading printer model previews: %w", err)
		}
		return models, nil
	}
	return s.RefreshAllModels(ctx)
}

func (s *PrinterService) RefreshAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
	printers, err := s.printers.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing printers: %w", err)
	}
	var all []domain.PrinterModelFile
	for _, printer := range printers {
		if !printer.Enabled || printer.Kind != domain.PrinterKindMoonraker {
			continue
		}
		if printer.State == domain.PrinterStateOffline {
			if s.models == nil {
				continue
			}
			cached, err := s.models.List(ctx, &printer.ID)
			if err != nil {
				return nil, fmt.Errorf("listing cached models for offline printer %q: %w", printer.Name, err)
			}
			all = append(all, cached...)
			continue
		}
		models, err := s.RefreshModels(ctx, printer.ID)
		if err != nil {
			if s.models != nil {
				cached, cacheErr := s.models.List(ctx, &printer.ID)
				if cacheErr != nil {
					return nil, fmt.Errorf("listing cached models for printer %q after refresh failure: %w", printer.Name, cacheErr)
				}
				all = append(all, cached...)
			}
			continue
		}
		all = append(all, models...)
	}
	if err := s.previews.HydrateModels(ctx, all); err != nil {
		return nil, fmt.Errorf("loading printer model previews: %w", err)
	}
	return all, nil
}

func (s *PrinterService) SyncPrinterHistory(ctx context.Context, printerID string, limit int) (int, error) {
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

	importJobs := func(remoteJobs []domain.PrinterHistoryJob) (int, error) {
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
			remoteJobs[index].PrinterID = printerID
			s.enrichHistoryJob(ctx, *printer, &remoteJobs[index])
		})

		created := 0
		for _, job := range remoteJobs {
			if job.ExternalPrintID == "" {
				continue
			}
			if local, ok := existingJobs[job.ExternalPrintID]; ok {
				job.PrinterID = printerID
				if local.PreviewImageHash == "" && len(job.PreviewImageData) > 0 && s.previews != nil {
					hash, path, storeErr := s.previews.Store(ctx, job.PreviewImageData)
					if storeErr != nil {
						return created, fmt.Errorf("storing recovered print preview image: %w", storeErr)
					}
					if hash != "" {
						updater, ok := s.prints.(port.PrintJobPreviewUpdater)
						if !ok {
							return created, fmt.Errorf("print job repository cannot update preview images")
						}
						if err := updater.UpdatePreviewImageHash(ctx, local.ID, hash); err != nil {
							return created, err
						}
						local.PreviewImageHash = hash
						local.PreviewImagePath = path
						existingJobs[job.ExternalPrintID] = local
					}
				}
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

func parallelFor(ctx context.Context, count, limit int, work func(int)) {
	if count == 0 {
		return
	}
	if limit <= 0 || limit > count {
		limit = count
	}

	indexes := make(chan int)
	var wg sync.WaitGroup
	for range limit {
		wg.Go(func() {
			for index := range indexes {
				work(index)
			}
		})
	}
	for index := range count {
		select {
		case <-ctx.Done():
			close(indexes)
			wg.Wait()
			return
		case indexes <- index:
		}
	}
	close(indexes)
	wg.Wait()
}

func (s *PrinterService) AssignHistoryJobSpool(ctx context.Context, printerID, externalPrintID, spoolID string) (*domain.PrintJob, error) {
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

func (s *PrinterService) StartPrinterFile(ctx context.Context, req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
	if s.jobClient == nil {
		return nil, fmt.Errorf("printer job client is not configured")
	}
	printer, err := s.printers.GetByID(ctx, req.PrinterID)
	if err != nil {
		return nil, fmt.Errorf("getting printer: %w", err)
	}
	metadata, err := s.jobClient.FetchMetadata(ctx, *printer, req.GcodeFilename)
	if err != nil {
		return nil, fmt.Errorf("fetching printer file metadata: %w", err)
	}
	check, err := s.checkFilament(ctx, *printer, req.SpoolID, metadata)
	if err != nil {
		return nil, err
	}
	result := &domain.PrintLaunchResult{Filename: req.GcodeFilename, Filament: check}
	if check.Blocked {
		return result, nil
	}
	if check.Warning != "" && !req.AllowLowFilament {
		result.RequiresUserConfirmation = true
		return result, nil
	}
	if err := s.jobClient.StartFile(ctx, *printer, req.GcodeFilename); err != nil {
		return nil, err
	}
	result.Started = true
	return result, nil
}

func (s *PrinterService) PrintModel(ctx context.Context, req domain.PrintLaunchRequest) (launchResult *domain.PrintLaunchResult, err error) {
	if req.SourcePrinterID == "" {
		req.SourcePrinterID = req.PrinterID
	}
	if req.SourcePrinterID == req.PrinterID {
		return s.StartPrinterFile(ctx, req)
	}
	if s.jobClient == nil {
		return nil, fmt.Errorf("printer job client is not configured")
	}
	source, err := s.printers.GetByID(ctx, req.SourcePrinterID)
	if err != nil {
		return nil, fmt.Errorf("getting source printer: %w", err)
	}
	target, err := s.printers.GetByID(ctx, req.PrinterID)
	if err != nil {
		return nil, fmt.Errorf("getting target printer: %w", err)
	}
	metadata, err := s.jobClient.FetchMetadata(ctx, *source, req.GcodeFilename)
	if err != nil {
		return nil, fmt.Errorf("fetching source model metadata: %w", err)
	}
	check, err := s.checkFilament(ctx, *target, req.SpoolID, metadata)
	if err != nil {
		return nil, err
	}
	result := &domain.PrintLaunchResult{Filename: req.GcodeFilename, Filament: check}
	if check.Blocked {
		return result, nil
	}
	if check.Warning != "" && !req.AllowLowFilament {
		result.RequiresUserConfirmation = true
		return result, nil
	}
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	targetModels, err := s.jobClient.ListModels(ctx, *target)
	if err != nil {
		return nil, fmt.Errorf("listing target printer models: %w", err)
	}
	remotePath := availableModelPath(req.GcodeFilename, targetModels)
	tmpDir, err := os.MkdirTemp("", "filatrack-transfer-*")
	if err != nil {
		return nil, fmt.Errorf("creating transfer cache: %w", err)
	}
	defer func(path string) {
		osErr := os.RemoveAll(path)
		if osErr != nil {
			s.logger.Error("removing transfer cache failed", "path", path, "error", osErr)
		}
	}(tmpDir)
	tmpPath := filepath.Join(tmpDir, filepath.Base(req.GcodeFilename))
	if err := s.jobClient.DownloadFile(ctx, *source, req.GcodeFilename, tmpPath); err != nil {
		return nil, err
	}
	uploaded, err := s.jobClient.Upload(ctx, *target, tmpPath, remotePath, true)
	if err != nil {
		return nil, err
	}
	result.Filename = uploaded.Path
	result.Uploaded = true
	result.Started = uploaded.PrintStarted
	result.Queued = uploaded.PrintQueued
	return result, nil
}

func (s *PrinterService) UploadLocalFile(ctx context.Context, req domain.PrintLaunchRequest, startPrint bool) (*domain.PrintLaunchResult, error) {
	if s.jobClient == nil {
		return nil, fmt.Errorf("printer job client is not configured")
	}
	if strings.TrimSpace(req.LocalPath) == "" {
		return nil, fmt.Errorf("local gcode path is required")
	}
	printer, err := s.printers.GetByID(ctx, req.PrinterID)
	if err != nil {
		return nil, fmt.Errorf("getting printer: %w", err)
	}
	metadata, err := gcode.ParseFile(req.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("parsing local gcode: %w", err)
	}
	if metadata.Filename == "" {
		metadata.Filename = filepath.Base(req.LocalPath)
	}
	result := &domain.PrintLaunchResult{Filename: metadata.Filename}
	if startPrint {
		check, err := s.checkFilament(ctx, *printer, req.SpoolID, metadata)
		if err != nil {
			return nil, err
		}
		result.Filament = check
		if check.Blocked {
			return result, nil
		}
		if check.Warning != "" && !req.AllowLowFilament {
			result.RequiresUserConfirmation = true
			return result, nil
		}
	}
	uploaded, err := s.jobClient.Upload(ctx, *printer, req.LocalPath, filepath.Base(req.LocalPath), startPrint)
	if err != nil {
		return nil, err
	}
	result.Filename = uploaded.Path
	result.Uploaded = true
	result.Started = uploaded.PrintStarted
	result.Queued = uploaded.PrintQueued
	if err := s.cacheUploadedModel(ctx, printer.ID, uploaded.Path, req.LocalPath, metadata); err != nil {
		s.logger.Warn("caching uploaded model metadata failed", "printer_id", printer.ID, "path", uploaded.Path, "error", err)
	}
	return result, nil
}

func availableModelPath(requested string, models []domain.PrinterModelFile) string {
	occupied := make(map[string]struct{}, len(models))
	for _, model := range models {
		occupied[model.Path] = struct{}{}
	}
	if _, exists := occupied[requested]; !exists {
		return requested
	}
	folder, filename := path.Split(requested)
	ext := path.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	for copyNumber := 2; ; copyNumber++ {
		candidate := folder + stem + " (" + strconv.Itoa(copyNumber) + ")" + ext
		if _, exists := occupied[candidate]; !exists {
			return candidate
		}
	}
}

func (s *PrinterService) cacheUploadedModel(ctx context.Context, printerID, uploadedPath, localPath string, metadata *domain.GcodeMetadata) error {
	if metadata == nil {
		return nil
	}
	imageHash, imagePath := metadata.PreviewImageHash, metadata.PreviewImagePath
	if len(metadata.PreviewImageData) > 0 && s.previews != nil {
		var err error
		imageHash, imagePath, err = s.previews.Store(ctx, metadata.PreviewImageData)
		if err != nil {
			return err
		}
	}
	if s.models == nil {
		return nil
	}
	if uploadedPath == "" {
		uploadedPath = filepath.Base(localPath)
	}
	models, err := s.models.List(ctx, &printerID)
	if err != nil {
		return err
	}
	model := domain.PrinterModelFile{
		PrinterID:        printerID,
		Path:             uploadedPath,
		Filename:         filepath.Base(uploadedPath),
		EstimatedWeightG: metadata.FilamentWeightG,
		PrintDuration:    metadata.PrintDuration,
		Slicer:           metadata.Slicer,
		PreviewImagePath: imagePath,
		PreviewImageHash: imageHash,
		CachedAt:         time.Now(),
	}
	if info, statErr := os.Stat(localPath); statErr == nil {
		model.ModifiedAt = info.ModTime()
		model.SizeBytes = info.Size()
	}
	updated := false
	for i := range models {
		if models[i].Path == uploadedPath {
			models[i] = model
			updated = true
			break
		}
	}
	if !updated {
		models = append(models, model)
	}
	return s.models.SaveForPrinter(ctx, printerID, models)
}

func (s *PrinterService) updateExistingHistoryUsage(ctx context.Context, local domain.PrintJob, item domain.PrinterHistoryJob) error {
	estimatedWeight := local.EstimatedWeightG
	if item.EstimatedWeightG > 0 {
		estimatedWeight = item.EstimatedWeightG
	}
	estimatedLength := local.EstimatedLengthMM
	if item.FilamentUsedMM > 0 {
		estimatedLength = item.FilamentUsedMM
	}
	actualWeight := local.ActualWeightG
	completion := local.CompletionPercent

	var spool *domain.Spool
	if local.SpoolID != "" {
		loaded, err := s.spools.GetByID(ctx, local.SpoolID)
		if err != nil {
			return fmt.Errorf("getting print job spool: %w", err)
		}
		spool = loaded
	}
	if estimatedLength > 0 && spool != nil {
		actualWeight = domain.WeightFromLength(estimatedLength, spool.DiameterMM, spool.DensityGCM3)
		if estimatedWeight == 0 && local.Status == domain.PrintStatusSuccess {
			estimatedWeight = actualWeight
		}
	}
	if status, ok := item.PrintStatus(); ok {
		if status == domain.PrintStatusSuccess {
			completion = 100
			if actualWeight == 0 && estimatedWeight > 0 {
				actualWeight = estimatedWeight
			}
		} else if estimatedWeight > 0 && actualWeight > 0 {
			completion = min(max(int(actualWeight/estimatedWeight*100), 0), 100)
		}
	}

	if estimatedWeight == local.EstimatedWeightG &&
		actualWeight == local.ActualWeightG &&
		estimatedLength == local.EstimatedLengthMM &&
		completion == local.CompletionPercent &&
		(item.FilamentUsedMM <= 0 || local.UsageMeasured) {
		return nil
	}
	return s.prints.UpdateUsage(ctx, local.ID, estimatedWeight, actualWeight, estimatedLength, completion, local.UsageMeasured || (item.FilamentUsedMM > 0 && spool != nil))
}

func (s *PrinterService) createPrintFromHistory(ctx context.Context, item domain.PrinterHistoryJob, spool *domain.Spool) (*domain.PrintJob, error) {
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

func (s *PrinterService) enrichHistoryJob(ctx context.Context, printer domain.Printer, job *domain.PrinterHistoryJob) {
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

func (s *PrinterService) checkFilament(ctx context.Context, printer domain.Printer, spoolID string, metadata *domain.GcodeMetadata) (domain.FilamentCheck, error) {
	if spoolID == "" {
		spoolID = printer.ActiveSpoolID
	}
	if spoolID == "" {
		return domain.FilamentCheck{}, fmt.Errorf("spool is required before starting a print")
	}
	spool, err := s.spools.GetByID(ctx, spoolID)
	if err != nil {
		return domain.FilamentCheck{}, fmt.Errorf("getting spool: %w", err)
	}
	used, err := s.spools.GetUsedWeight(ctx, spoolID)
	if err != nil {
		return domain.FilamentCheck{}, fmt.Errorf("getting spool used weight: %w", err)
	}
	required := metadata.FilamentWeightG
	if required == 0 && metadata.FilamentLengthMM > 0 {
		required = domain.WeightFromLength(metadata.FilamentLengthMM, spool.DiameterMM, spool.DensityGCM3)
	}
	remaining := spool.RemainingWeightG(used)
	check := domain.FilamentCheck{
		RequiredWeightG:  required,
		RemainingBeforeG: remaining,
		RemainingAfterG:  remaining - required,
	}
	if check.RemainingAfterG < 0 {
		check.RemainingAfterG = 0
	}
	if required > remaining {
		check.Blocked = true
		check.Warning = fmt.Sprintf("not enough filament: need %.1fg, remaining %.1fg", required, remaining)
		return check, nil
	}
	rawLevel, err := s.settings.GetSetting(ctx, lowFilamentLevelSettingKey)
	if err != nil {
		return check, fmt.Errorf("getting low filament level: %w", err)
	}

	lowLevelValue, err := strconv.ParseFloat(rawLevel, 64)
	if err != nil {
		return check, fmt.Errorf("parsing low filament level %q: %w", rawLevel, err)
	}
	if lowLevelValue < 0 {
		return check, fmt.Errorf("low filament level must not be negative")
	}

	if check.RemainingAfterG < lowLevelValue {
		check.Warning = fmt.Sprintf(
			"low filament after print: %.1fg remaining",
			check.RemainingAfterG,
		)
	}
	return check, nil
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

func (s *PrinterService) findRecordedPrinterJob(ctx context.Context, printerID, externalID string) (*domain.PrintJob, error) {
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

func (s *PrinterService) assignRecordedJobSpool(ctx context.Context, jobID string, spool *domain.Spool) (*domain.PrintJob, error) {
	if err := s.prints.UpdateSpool(ctx, jobID, spool.ID); err != nil {
		return nil, fmt.Errorf("changing print job spool: %w", err)
	}
	job, err := s.prints.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.EstimatedLengthMM > 0 {
		actualWeight := domain.WeightFromLength(job.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
		estimatedWeight := job.EstimatedWeightG
		if estimatedWeight == 0 && job.Status == domain.PrintStatusSuccess {
			estimatedWeight = actualWeight
		}
		completion := job.CompletionPercent
		if completion <= 0 && estimatedWeight > 0 {
			completion = min(max(int(actualWeight/estimatedWeight*100), 0), 100)
		}
		if err := s.prints.UpdateUsage(ctx, job.ID, estimatedWeight, actualWeight, job.EstimatedLengthMM, completion, true); err != nil {
			return nil, fmt.Errorf("updating print job usage: %w", err)
		}
		job.EstimatedWeightG = estimatedWeight
		job.ActualWeightG = actualWeight
		job.CompletionPercent = completion
		job.UsageMeasured = true
	}
	job.PreviewImagePath, err = s.previews.Resolve(ctx, job.PreviewImageHash, job.PreviewImagePath)
	if err != nil {
		return nil, fmt.Errorf("loading print preview image: %w", err)
	}
	return job, nil
}
