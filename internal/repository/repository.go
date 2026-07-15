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
	GetStats(ctx context.Context, filter StatsFilter) (*PrintStats, error)
}

type PrintJobFilter struct {
	SpoolID *string
	Status  *domain.PrintStatus
	From    *time.Time
	To      *time.Time
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
	ListCustomMaterials(ctx context.Context) ([]string, error)
	AddCustomMaterial(ctx context.Context, name string) error
	DeleteCustomMaterial(ctx context.Context, name string) error
}
