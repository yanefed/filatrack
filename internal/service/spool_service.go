package service

import (
	"context"
	"fmt"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
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
		spool.DiameterMM = 1.75
	}
	if spool.DensityGCM3 <= 0 {
		if d, ok := domain.DefaultDensities[spool.Material]; ok {
			spool.DensityGCM3 = d
		} else {
			spool.DensityGCM3 = 1.24
		}
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
		spool.DiameterMM = 1.75
	}
	if spool.DensityGCM3 <= 0 {
		if d, ok := domain.DefaultDensities[spool.Material]; ok {
			spool.DensityGCM3 = d
		} else {
			spool.DensityGCM3 = 1.24
		}
	}
	return s.spools.Update(ctx, spool)
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
		profiles, err := s.settings.ListDryingProfiles(ctx)
		if err == nil {
			for _, p := range profiles {
				if p.Material == spool.Material {
					last, err := s.dryings.GetLastDrying(ctx, spool.ID)
					if err != nil {
						return false, err
					}
					var lastDried time.Time
					if last != nil {
						lastDried = last.DriedAt
					}
					return domain.NeedsDrying(lastDried, p), nil
				}
			}
		}
	}

	// Fallback to defaults
	profile, ok := domain.DefaultDryingProfiles[spool.Material]
	if !ok {
		return false, nil
	}
	last, err := s.dryings.GetLastDrying(ctx, spool.ID)
	if err != nil {
		return false, err
	}
	var lastDried time.Time
	if last != nil {
		lastDried = last.DriedAt
	}
	return domain.NeedsDrying(lastDried, profile), nil
}
