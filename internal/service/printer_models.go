package service

import (
	"context"
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
	logger    *slog.Logger
	printers  port.PrinterRepository
	spools    port.SpoolRepository
	models    port.PrinterModelRepository
	previews  *PreviewImageStore
	jobClient port.PrinterJobClient
}

func (s *printerModels) ListModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
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

func (s *printerModels) RefreshModels(ctx context.Context, printerID string) ([]domain.PrinterModelFile, error) {
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

func (s *printerModels) ListAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
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

func (s *printerModels) RefreshAllModels(ctx context.Context) ([]domain.PrinterModelFile, error) {
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
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
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

func (s *printerModels) cacheUploadedModel(ctx context.Context, printerID, uploadedPath, localPath string, metadata *domain.GcodeMetadata) error {
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
