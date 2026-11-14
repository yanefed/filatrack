package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

type printerManagement struct {
	printers port.PrinterRepository
	checker  port.ConnectionChecker
	spools   port.SpoolRepository
}

func (s *printerManagement) CreateMoonrakerPrinter(ctx context.Context, printer *domain.Printer) error {
	if err := normalizePrinter(printer); err != nil {
		return err
	}
	now := time.Now()
	printer.Kind = domain.PrinterKindMoonraker
	printer.Enabled = true
	printer.State = domain.PrinterStateUnknown
	printer.CreatedAt = now
	printer.UpdatedAt = now
	return s.printers.Create(ctx, printer)
}

func (s *printerManagement) List(ctx context.Context) ([]domain.Printer, error) {
	return s.printers.List(ctx)
}

func (s *printerManagement) CheckMoonrakerConnection(ctx context.Context, printer *domain.Printer) error {
	if printer == nil {
		return fmt.Errorf("printer is required")
	}
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

func (s *printerManagement) UpdateMoonrakerPrinter(ctx context.Context, printer *domain.Printer) error {
	if err := normalizePrinter(printer); err != nil {
		return err
	}
	if err := s.CheckMoonrakerConnection(ctx, printer); err != nil {
		return fmt.Errorf("checking printer connection: %w", err)
	}
	return s.printers.Update(ctx, printer)
}

func (s *printerManagement) AssignSpool(ctx context.Context, printerID, spoolID string) error {
	if _, err := s.spools.GetByID(ctx, spoolID); err != nil {
		return fmt.Errorf("getting spool: %w", err)
	}
	return s.printers.AssignSpool(ctx, printerID, spoolID)
}

func (s *printerManagement) ClearAssignedSpool(ctx context.Context, printerID string) error {
	return s.printers.AssignSpool(ctx, printerID, "")
}

func (s *printerManagement) UpdateStatus(ctx context.Context, printer *domain.Printer) error {
	if printer == nil {
		return fmt.Errorf("printer is required")
	}
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

func normalizePrinter(printer *domain.Printer) error {
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
	return nil
}
