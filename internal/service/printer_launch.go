package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/gcode"
	"github.com/yanefed/filatrack/internal/port"
)

type printerLaunch struct {
	transferMu sync.Mutex
	logger     *slog.Logger
	printers   port.PrinterRepository
	spools     port.SpoolRepository
	settings   port.SettingsRepository
	jobClient  port.PrinterJobClient
	catalog    *printerModels
}

func (s *printerLaunch) StartPrinterFile(ctx context.Context, req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
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
	if requireLaunchConfirmation(result, req.AllowLowFilament) {
		return result, nil
	}
	if err := s.jobClient.StartFile(ctx, *printer, req.GcodeFilename); err != nil {
		return nil, err
	}
	result.Started = true
	return result, nil
}

func (s *printerLaunch) PrintModel(ctx context.Context, req domain.PrintLaunchRequest) (*domain.PrintLaunchResult, error) {
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
	if requireLaunchConfirmation(result, req.AllowLowFilament) {
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

func (s *printerLaunch) UploadLocalFile(ctx context.Context, req domain.PrintLaunchRequest, startPrint bool) (*domain.PrintLaunchResult, error) {
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
		if requireLaunchConfirmation(result, req.AllowLowFilament) {
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
	if err := s.catalog.cacheUploadedModel(ctx, printer.ID, uploaded.Path, req.LocalPath, metadata); err != nil {
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

func (s *printerLaunch) checkFilament(ctx context.Context, printer domain.Printer, spoolID string, metadata *domain.GcodeMetadata) (domain.FilamentCheck, error) {
	if metadata == nil {
		return domain.FilamentCheck{}, fmt.Errorf("gcode metadata is required")
	}
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
	lowLevelValue, err := readLowFilamentLevel(ctx, s.settings)
	if err != nil {
		return check, err
	}

	if check.RemainingAfterG < float64(lowLevelValue) {
		check.Warning = fmt.Sprintf(
			"low filament after print: %.1fg remaining",
			check.RemainingAfterG,
		)
	}
	return check, nil
}

// requireLaunchConfirmation also stops blocked launches, which cannot be overridden.
func requireLaunchConfirmation(result *domain.PrintLaunchResult, allowLowFilament bool) bool {
	result.RequiresUserConfirmation = !result.Filament.Blocked && result.Filament.Warning != "" && !allowLowFilament
	return result.Filament.Blocked || result.RequiresUserConfirmation
}
