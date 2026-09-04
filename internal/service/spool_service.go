package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
	"context"
	"fmt"
	"time"
)

type SpoolService struct {
	spools   repository.SpoolRepository
	dryings  repository.DryingSessionRepository
	settings repository.SettingsRepository
}

func NewSpoolService(spools repository.SpoolRepository, dryings repository.DryingSessionRepository, settings repository.SettingsRepository) *SpoolService {
	return &SpoolService{spools: spools, dryings: dryings, settings: settings}
}

func (s *SpoolService) Create(ctx context.Context, spool *domain.Spool) error {
	if spool.TotalWeightG <= 0 {
		return fmt.Errorf("total weight must be positive")
	}
	if spool.DiameterMM <= 0 {
		spool.DiameterMM = domain.DefaultDiameter
	}
	if spool.DensityGCM3 <= 0 {
		spool.DensityGCM3 = s.materialDensity(ctx, spool.Material)
	}
	now := time.Now()
	spool.CreatedAt = now
	spool.UpdatedAt = now
	return s.spools.Create(ctx, spool)
}

func (s *SpoolService) Update(ctx context.Context, spool *domain.Spool) error {
	if spool.ID == "" {
		return fmt.Errorf("spool id is required")
	}
	if spool.TotalWeightG <= 0 {
		return fmt.Errorf("total weight must be positive")
	}
	if spool.DiameterMM <= 0 {
		spool.DiameterMM = domain.DefaultDiameter
	}
	if spool.DensityGCM3 <= 0 {
		spool.DensityGCM3 = s.materialDensity(ctx, spool.Material)
	}
	return s.spools.Update(ctx, spool)
}

func (s *SpoolService) materialDensity(ctx context.Context, material domain.Material) float64 {
	if s.settings != nil {
		definition, err := s.settings.GetMaterial(ctx, material)
		if err == nil && definition != nil && definition.DensityGCM3 > 0 {
			return definition.DensityGCM3
		}
	}
	return 1.24
}

func (s *SpoolService) Archive(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("spool id is required")
	}
	return s.spools.Archive(ctx, id)
}

func (s *SpoolService) GetWithRemaining(ctx context.Context, id string) (*domain.Spool, float64, error) {
	spool, err := s.spools.GetByID(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	used, err := s.spools.GetUsedWeight(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	return spool, spool.RemainingWeightG(used), nil
}

func (s *SpoolService) List(ctx context.Context, filter repository.SpoolFilter) ([]domain.Spool, error) {
	return s.spools.List(ctx, filter)
}

func (s *SpoolService) NeedsDrying(ctx context.Context, spool *domain.Spool) (bool, error) {
	// Try DB profiles first
	if s.settings != nil {
		profile, err := s.settings.GetDryingProfileForMaterial(ctx, spool.Material)
		if err == nil && profile != nil {
			last, err := s.dryings.GetLastDrying(ctx, spool.ID)
			if err != nil {
				return false, err
			}
			var lastDried time.Time
			if last != nil {
				lastDried = last.DriedAt
			}
			return domain.NeedsDrying(lastDried, *profile), nil
		}
	}

	return false, nil
}
