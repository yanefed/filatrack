package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/gcode"
	"github.com/yanefed/filatrack/internal/port"
)

type printerModels struct {
	logger        *slog.Logger
	printers      port.PrinterRepository
	spools        port.SpoolRepository
	models        port.PrinterModelRepository
	previews      *PreviewImageStore
	jobClient     port.PrinterJobClient
	locks         sync.Map // printer ID -> cancelable catalog lock
	printerSlots  chan struct{}
	metadataSlots chan struct{}
}

func (s *printerModels) lockPrinter(ctx context.Context, id string) (func(), error) {
	value, _ := s.locks.LoadOrStore(id, make(chan struct{}, 1))
	lock := value.(chan struct{})
	select {
	case lock <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-lock
			return nil, err
		}
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *printerModels) ListModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
	if s.models == nil {
		return s.RefreshModels(ctx, printerID)
	}
	models, err := s.models.List(ctx, &printerID)
	if err != nil {
		return nil, err
	}
	if err := s.presentModels(ctx, models); err != nil {
		return nil, err
	}
	return models, nil
}

func (s *printerModels) ListAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
	if s.models == nil {
		return s.RefreshAllModels(ctx)
	}
	models, err := s.models.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := s.presentModels(ctx, models); err != nil {
		return nil, err
	}
	return models, nil
}

// RefreshModels is the single-printer convenience API. Partial results accompany errors.
func (s *printerModels) RefreshModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
	printer, err := s.printers.GetByID(ctx, printerID)
	if err != nil {
		return nil, err
	}
	result, err := s.refreshPrinterModels(ctx, *printer, ModelRefreshChanges)
	if err != nil {
		return nil, err
	}
	if err := s.presentModels(ctx, result.Models); err != nil {
		return result.Models, err
	}
	return result.Models, refreshFailuresError(result.Failures)
}

func refreshFailuresError(failures []ModelRefreshFailure) error {
	errs := make([]error, len(failures))
	for i := range failures {
		errs[i] = failures[i]
	}
	return errors.Join(errs...)
}

func (s *printerModels) RefreshAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
	result, err := s.RefreshModelCatalog(ctx, ModelRefreshChanges)
	if err != nil {
		return nil, err
	}
	return result.Models, refreshFailuresError(result.Failures)
}

// RefreshModelCatalog returns per-printer/file failures separately from operation
// errors (cancellation, repository access or presentation). Successful printers commit independently.
func (s *printerModels) RefreshModelCatalog(ctx context.Context, mode ModelRefreshMode) (ModelRefreshResult, error) {
	if mode != ModelRefreshChanges && mode != ModelRefreshFull {
		return ModelRefreshResult{}, fmt.Errorf("invalid model refresh mode: %d", mode)
	}
	printers, err := s.printers.List(ctx)
	if err != nil {
		return ModelRefreshResult{}, fmt.Errorf("listing printers: %w", err)
	}
	results := make([]ModelRefreshResult, len(printers))
	errs := make([]error, len(printers))
	parallelFor(ctx, len(printers), 2, func(i int) {
		printer := printers[i]
		if !printer.Enabled || printer.Kind != domain.PrinterKindMoonraker {
			return
		}
		results[i], errs[i] = s.refreshPrinterModels(ctx, printer, mode)
	})
	var result ModelRefreshResult
	for i := range results {
		result.Models = append(result.Models, results[i].Models...)
		result.Failures = append(result.Failures, results[i].Failures...)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := errors.Join(errs...); err != nil {
		return result, err
	}
	if err := s.presentModels(ctx, result.Models); err != nil {
		return result, err
	}
	return result, nil
}

func (s *printerModels) refreshPrinterModels(ctx context.Context, printer domain.Printer, mode ModelRefreshMode) (ModelRefreshResult, error) {
	unlock, err := s.lockPrinter(ctx, printer.ID)
	if err != nil {
		return ModelRefreshResult{}, err
	}
	defer unlock()
	var cached []domain.PrinterModelFile
	if s.models != nil {
		cached, err = s.models.List(ctx, &printer.ID)
		if err != nil {
			return ModelRefreshResult{}, fmt.Errorf("reading model cache for %s: %w", printer.Name, err)
		}
	}
	result := ModelRefreshResult{Models: cached}
	failed := func(err error) (ModelRefreshResult, error) {
		result.Failures = append(result.Failures, ModelRefreshFailure{PrinterID: printer.ID, PrinterName: printer.Name, Err: err})
		return result, nil
	}
	if printer.State == domain.PrinterStateOffline {
		return failed(fmt.Errorf("printer %q is offline", printer.Name))
	}
	if s.jobClient == nil {
		return failed(fmt.Errorf("printer job client is not configured"))
	}
	select {
	case s.printerSlots <- struct{}{}:
		defer func() { <-s.printerSlots }()
	case <-ctx.Done():
		return result, ctx.Err()
	}
	files, err := s.jobClient.ListModels(ctx, printer)
	if err != nil {
		return failed(err)
	}
	cachedByPath := make(map[string]domain.PrinterModelFile, len(cached))
	for _, model := range cached {
		cachedByPath[model.Path] = model
	}
	failures := make([]error, len(files))
	changed := make([]bool, len(files))
	parallelFor(ctx, len(files), metadataSyncConcurrency, func(i int) {
		file := files[i]
		file.PrinterID = printer.ID
		previous, found := cachedByPath[file.Path]
		same := found && sameModelVersion(previous, file, printer.MoonrakerURL)
		if same && previous.MetadataReady && mode == ModelRefreshChanges {
			files[i] = previous
			return
		}
		changed[i] = true
		select {
		case s.metadataSlots <- struct{}{}:
			defer func() { <-s.metadataSlots }()
		case <-ctx.Done():
			return
		}
		files[i], failures[i] = s.enrichModel(ctx, printer, file, previous, same, mode)
	})
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for i, err := range failures {
		if err != nil {
			result.Failures = append(result.Failures, ModelRefreshFailure{PrinterID: printer.ID, PrinterName: printer.Name, Path: files[i].Path, Err: err})
		}
	}
	catalogChanged := len(files) != len(cached)
	for _, fileChanged := range changed {
		catalogChanged = catalogChanged || fileChanged
	}
	if !catalogChanged {
		// Retain repository ordering and avoid rewriting an unchanged catalog.
		return result, nil
	}
	if s.models != nil {
		if err := s.models.SaveForPrinter(ctx, printer.ID, files); err != nil {
			return result, err
		}
		// Preserve repository ordering and timestamps, including the empty-catalog case.
		files, err = s.models.List(ctx, &printer.ID)
		if err != nil {
			return result, err
		}
	}
	result.Models = files
	return result, nil
}

func sameModelVersion(cached, remote domain.PrinterModelFile, sourceURL string) bool {
	return cached.SourceURL == sourceURL && !remote.ModifiedAt.IsZero() &&
		cached.ModifiedAt.Equal(remote.ModifiedAt) && cached.SizeBytes == remote.SizeBytes
}

func (s *printerModels) enrichModel(ctx context.Context, printer domain.Printer, file, previous domain.PrinterModelFile, same bool, mode ModelRefreshMode) (domain.PrinterModelFile, error) {
	file.SourceURL = printer.MoonrakerURL
	file.CachedAt = time.Now()
	metadata, err := s.jobClient.FetchMetadata(ctx, printer, file.Path)
	if err == nil && metadata == nil {
		err = fmt.Errorf("empty metadata response")
	}
	if err != nil {
		if same {
			previous.MetadataReady = false
			return previous, err
		}
		return file, err
	}
	// Do not mutate adapter-owned responses; clients may share immutable metadata.
	copy := *metadata
	metadata = &copy
	if metadata.FilamentWeightG <= 0 && metadata.FilamentLengthMM <= 0 {
		if same && previous.GcodeParsed && mode == ModelRefreshChanges {
			metadata.MergeMissingGcodeMetadata(&domain.GcodeMetadata{
				FilamentWeightG: previous.FilamentWeightG, FilamentLengthMM: previous.FilamentLengthMM,
				PrintDuration: previous.PrintDuration, Slicer: previous.Slicer,
			})
			file.GcodeParsed = true
		} else {
			fallback, parseErr := s.downloadModelMetadata(ctx, printer, file.Path)
			if parseErr != nil {
				err = fmt.Errorf("reading gcode metadata: %w", parseErr)
			} else {
				metadata.MergeMissingGcodeMetadata(fallback)
				file.GcodeParsed = true
			}
		}
	}
	file.FilamentWeightG, file.FilamentLengthMM = metadata.FilamentWeightG, metadata.FilamentLengthMM
	file.EstimatedWeightG = metadata.FilamentWeightG
	file.PrintDuration, file.Slicer = metadata.PrintDuration, metadata.Slicer
	file.PreviewImagePath, file.PreviewImageHash = metadata.PreviewImagePath, metadata.PreviewImageHash
	if len(metadata.PreviewImageData) > 0 && s.previews != nil {
		hash, path, storeErr := s.previews.Store(ctx, metadata.PreviewImageData)
		if storeErr != nil {
			err = errors.Join(err, fmt.Errorf("storing model preview: %w", storeErr))
		} else {
			file.PreviewImageHash, file.PreviewImagePath = hash, path
		}
	}
	file.MetadataReady = err == nil
	return file, err
}

// presentModels computes spool-dependent weight without writing it back as slicer data.
func (s *printerModels) presentModels(ctx context.Context, models []domain.PrinterModelFile) error {
	properties := make(map[string][2]float64)
	for i := range models {
		model := &models[i]
		if model.FilamentWeightG > 0 {
			model.EstimatedWeightG = model.FilamentWeightG
		} else if model.FilamentLengthMM > 0 {
			props, ok := properties[model.PrinterID]
			if !ok {
				props = [2]float64{domain.DefaultDiameter, domain.DefaultDensity}
				printer, err := s.printers.GetByID(ctx, model.PrinterID)
				if err == nil && printer != nil {
					props[0], props[1] = s.modelFilamentProperties(ctx, printer)
				}
				properties[model.PrinterID] = props
			}
			model.EstimatedWeightG = domain.WeightFromLength(model.FilamentLengthMM, props[0], props[1])
		}
	}
	return s.previews.HydrateModels(ctx, models)
}
func (s *printerModels) modelFilamentProperties(ctx context.Context, printer *domain.Printer) (float64, float64) {
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

func (s *printerModels) downloadModelMetadata(ctx context.Context, printer domain.Printer, filename string) (*domain.GcodeMetadata, error) {
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

func (s *printerModels) cacheUploadedModel(ctx context.Context, printerID, uploadedPath, localPath string, metadata *domain.GcodeMetadata) error {
	if metadata == nil {
		return nil
	}
	unlock, err := s.lockPrinter(ctx, printerID)
	if err != nil {
		return err
	}
	defer unlock()
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
	printer, err := s.printers.GetByID(ctx, printerID)
	if err != nil {
		return err
	}
	model := domain.PrinterModelFile{
		SourceURL:       printer.MoonrakerURL,
		FilamentWeightG: metadata.FilamentWeightG, FilamentLengthMM: metadata.FilamentLengthMM,
		GcodeParsed:      true,
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
		model.SizeBytes = info.Size()
		// The server's mtime differs from the local file's. Record its actual
		// signature so the next refresh can reuse the already parsed G-code.
		if s.jobClient != nil {
			remote, listErr := s.jobClient.ListModels(ctx, *printer)
			if listErr != nil {
				s.logger.Debug("reading uploaded model signature failed", "printer_id", printerID, "error", listErr)
			} else {
				for _, file := range remote {
					if file.Path == uploadedPath && file.SizeBytes == model.SizeBytes {
						model.ModifiedAt = file.ModifiedAt
						break
					}
				}
			}
		}
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
