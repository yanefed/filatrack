package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/port"
	"context"
	"fmt"
	"slices"
	"time"
)

type SpoolService struct {
	spools   port.SpoolRepository
	dryings  port.DryingSessionRepository
	settings port.SettingsRepository
}

func NewSpoolService(spools port.SpoolRepository, dryings port.DryingSessionRepository, settings port.SettingsRepository) *SpoolService {
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

func (s *SpoolService) List(ctx context.Context, filter domain.SpoolFilter) ([]domain.Spool, error) {
	return s.spools.List(ctx, filter)
}

// FilterOptions includes all active spools, independent of the current selection.
func (s *SpoolService) FilterOptions(ctx context.Context) (SpoolFilterOptions, error) {
	archived := false
	spools, err := s.spools.List(ctx, domain.SpoolFilter{Archived: &archived})
	if err != nil {
		return SpoolFilterOptions{}, err
	}
	manufacturers := make(map[string]struct{})
	materials := make(map[domain.Material]struct{})
	for _, spool := range spools {
		manufacturers[spool.Manufacturer] = struct{}{}
		materials[spool.Material] = struct{}{}
	}
	options := SpoolFilterOptions{}
	for manufacturer := range manufacturers {
		options.Manufacturers = append(options.Manufacturers, manufacturer)
	}
	for material := range materials {
		options.Materials = append(options.Materials, material)
	}
	slices.Sort(options.Manufacturers)
	slices.Sort(options.Materials)
	return options, nil
}

func (s *SpoolService) NeedsDrying(ctx context.Context, spool *domain.Spool) (bool, error) {
	// Try DB profiles first
	if s.settings != nil {
		profile, err := s.settings.GetDryingProfileForMaterial(ctx, spool.Material)
		if err != nil {
			return false, fmt.Errorf("getting drying profile: %w", err)
		}
		if profile != nil {
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
