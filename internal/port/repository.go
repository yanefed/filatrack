package port

import (
	"context"
	"github.com/yanefed/filatrack/internal/domain"
)

type SpoolRepository interface {
	Create(ctx context.Context, spool *domain.Spool) error
	GetByID(ctx context.Context, id string) (*domain.Spool, error)
	List(ctx context.Context, filter domain.SpoolFilter) ([]domain.Spool, error)
	Update(ctx context.Context, spool *domain.Spool) error
	Archive(ctx context.Context, id string) error
	GetUsedWeight(ctx context.Context, spoolID string) (float64, error)
}

type PrintJobRepository interface {
	PrintJobPreviewUpdater
	// Create atomically rejects repeated (printer ID, external job ID) pairs
	// with domain.ErrPrintJobAlreadyExists; manual jobs have no external ID.
	Create(ctx context.Context, job *domain.PrintJob) error
	GetByID(ctx context.Context, id string) (*domain.PrintJob, error)
	List(ctx context.Context, filter domain.PrintJobFilter) ([]domain.PrintJob, error)
	UpdateSpool(ctx context.Context, jobID, spoolID string) error
	UpdateUsage(ctx context.Context, jobID string, estimatedWeightG, actualWeightG, estimatedLengthMM float64, completionPercent int, usageMeasured bool) error
	GetStats(ctx context.Context, filter domain.StatsFilter) (*domain.PrintStats, error)
}

type PrintJobPreviewUpdater interface {
	UpdatePreviewImageHash(ctx context.Context, jobID, hash string) error
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

type PreviewImageRepository interface {
	Put(ctx context.Context, data []byte) (string, error)
	Get(ctx context.Context, hash string) ([]byte, error)
}

type DryingSessionRepository interface {
	Create(ctx context.Context, session *domain.DryingSession) error
	List(ctx context.Context, spoolID string) ([]domain.DryingSession, error)
	GetLastDrying(ctx context.Context, spoolID string) (*domain.DryingSession, error)
}

type SettingsRepository interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	ListMaterials(ctx context.Context) ([]domain.MaterialDefinition, error)
	GetMaterial(ctx context.Context, name domain.Material) (*domain.MaterialDefinition, error)
	CreateMaterial(ctx context.Context, material *domain.MaterialDefinition) error
	UpdateMaterial(ctx context.Context, material *domain.MaterialDefinition) error
	DeleteMaterial(ctx context.Context, name domain.Material) error
	GetDryingProfileForMaterial(ctx context.Context, material domain.Material) (*domain.DryingProfile, error)
}
