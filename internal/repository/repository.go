package repository

import (
	"context"
	"time"

	"awesomeProject/internal/domain"
)

type SpoolRepository interface {
	Create(ctx context.Context, spool *domain.Spool) error
	GetByID(ctx context.Context, id string) (*domain.Spool, error)
	List(ctx context.Context, filter SpoolFilter) ([]domain.Spool, error)
	Update(ctx context.Context, spool *domain.Spool) error
	Archive(ctx context.Context, id string) error
	GetUsedWeight(ctx context.Context, spoolID string) (float64, error)
}

type SpoolFilter struct {
	Material     *domain.Material
	Manufacturer *string
	Color        *string
	Archived     *bool
}

type PrintJobRepository interface {
	Create(ctx context.Context, job *domain.PrintJob) error
	GetByID(ctx context.Context, id string) (*domain.PrintJob, error)
	List(ctx context.Context, filter PrintJobFilter) ([]domain.PrintJob, error)
	UpdateSpool(ctx context.Context, jobID, spoolID string) error
	UpdateUsage(ctx context.Context, jobID string, estimatedWeightG, actualWeightG, estimatedLengthMM float64, completionPercent int) error
	GetStats(ctx context.Context, filter StatsFilter) (*PrintStats, error)
}

type PrintJobFilter struct {
	SpoolID   *string
	PrinterID *string
	Status    *domain.PrintStatus
	From      *time.Time
	To        *time.Time
	Limit     int
}

type PrinterRepository interface {
	Create(ctx context.Context, printer *domain.Printer) error
	GetByID(ctx context.Context, id string) (*domain.Printer, error)
	List(ctx context.Context) ([]domain.Printer, error)
	Update(ctx context.Context, printer *domain.Printer) error
	AssignSpool(ctx context.Context, printerID, spoolID string) error
	UpdateStatus(ctx context.Context, printer *domain.Printer) error
}

type PrinterModelRepository interface {
	SaveForPrinter(ctx context.Context, printerID string, models []domain.PrinterModelFile) error
	List(ctx context.Context, printerID *string) ([]domain.PrinterModelFile, error)
}

type StatsFilter struct {
	From *time.Time
	To   *time.Time
}

type PrintStats struct {
	TotalPrints    int
	SuccessCount   int
	FailedCount    int
	CancelledCount int
	TotalWeightG   float64
	AvgWeightG     float64
}

type DryingSessionRepository interface {
	Create(ctx context.Context, session *domain.DryingSession) error
	List(ctx context.Context, spoolID string) ([]domain.DryingSession, error)
	GetLastDrying(ctx context.Context, spoolID string) (*domain.DryingSession, error)
}

type SettingsRepository interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	ListDryingProfiles(ctx context.Context) ([]domain.DryingProfile, error)
	UpsertDryingProfile(ctx context.Context, p *domain.DryingProfile) error
	DeleteDryingProfile(ctx context.Context, material string) error
	GetDryingProfileForMaterial(ctx context.Context, material domain.Material) (*domain.DryingProfile, error)
	ListCustomMaterials(ctx context.Context) ([]string, error)
	ListCustomMaterialBindings(ctx context.Context) ([]domain.CustomMaterial, error)
	AddCustomMaterial(ctx context.Context, name string) error
	SetCustomMaterialDryingProfile(ctx context.Context, name, profileMaterial string) error
	UpsertCustomMaterialDryingProfile(ctx context.Context, name string, profile *domain.DryingProfile) error
	DeleteCustomMaterial(ctx context.Context, name string) error
}
