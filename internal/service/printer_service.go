package service

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/gcode"
	"awesomeProject/internal/repository"
)

type PrinterService struct {
	printers repository.PrinterRepository
	spools   repository.SpoolRepository
	prints   repository.PrintJobRepository
	checker  ConnectionChecker
}

type ConnectionChecker interface {
	Check(ctx context.Context, printer domain.Printer) error
}

func NewPrinterService(printers repository.PrinterRepository, spools repository.SpoolRepository, prints repository.PrintJobRepository) *PrinterService {
	return &PrinterService{printers: printers, spools: spools, prints: prints}
}

func (s *PrinterService) SetConnectionChecker(checker ConnectionChecker) {
	s.checker = checker
}

func (s *PrinterService) CreateMoonrakerPrinter(ctx context.Context, printer *domain.Printer) error {
	if strings.TrimSpace(printer.Name) == "" {
		return fmt.Errorf("printer name is required")
	}
	if err := validateMoonrakerURL(printer.MoonrakerURL); err != nil {
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
	if err := validateMoonrakerURL(printer.MoonrakerURL); err != nil {
		return err
	}
	if s.checker == nil {
		return nil
	}
	candidate := *printer
	candidate.MoonrakerURL = strings.TrimRight(strings.TrimSpace(candidate.MoonrakerURL), "/")
	return s.checker.Check(ctx, candidate)
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

	status := event.Status
	if status == "" {
		status = domain.PrintStatusSuccess
	}
	completion := event.CompletionPercent
	if status == domain.PrintStatusSuccess || completion <= 0 {
		completion = 100
	}

	estimatedWeight := event.EstimatedWeightG
	if estimatedWeight == 0 && event.EstimatedLengthMM > 0 {
		estimatedWeight = gcode.WeightFromLength(event.EstimatedLengthMM, spool.DiameterMM, spool.DensityGCM3)
	}
	actualWeight := event.ActualWeightG
	if actualWeight == 0 {
		actualWeight = estimatedWeight * float64(completion) / 100
	}

	printedAt := event.PrintedAt
	if printedAt.IsZero() {
		printedAt = time.Now()
	}

	job := &domain.PrintJob{
		SpoolID:           printer.ActiveSpoolID,
		PrinterID:         printer.ID,
		ExternalPrintID:   event.ExternalPrintID,
		GcodeFilename:     event.GcodeFilename,
		ModelName:         event.ModelName,
		Status:            status,
		CompletionPercent: completion,
		EstimatedWeightG:  estimatedWeight,
		ActualWeightG:     actualWeight,
		EstimatedLengthMM: event.EstimatedLengthMM,
		PrintDuration:     event.PrintDuration,
		NozzleTempC:       event.NozzleTempC,
		BedTempC:          event.BedTempC,
		Slicer:            event.Slicer,
		PrintedAt:         printedAt,
		CreatedAt:         time.Now(),
	}
	if job.ModelName == "" {
		job.ModelName = modelNameFromGcode(event.GcodeFilename)
	}

	if err := s.prints.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating printer print job: %w", err)
	}
	return job, nil
}

func validateMoonrakerURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("valid moonraker URL is required")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("moonraker URL must use http, https, ws, or wss")
	}
	return nil
}

func modelNameFromGcode(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}
