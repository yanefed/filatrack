package service

import (
	"context"
	"fmt"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository"
)

type DryingService struct {
	dryings  repository.DryingSessionRepository
	settings repository.SettingsRepository
}

func NewDryingService(dryings repository.DryingSessionRepository, settings repository.SettingsRepository) *DryingService {
	return &DryingService{dryings: dryings, settings: settings}
}

func (s *DryingService) Create(ctx context.Context, session *domain.DryingSession) error {
	if session.TemperatureC <= 0 {
		return fmt.Errorf("temperature must be positive")
	}
	if session.DurationHours <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if session.DriedAt.IsZero() {
		session.DriedAt = time.Now()
	}
	session.CreatedAt = time.Now()
	return s.dryings.Create(ctx, session)
}

func (s *DryingService) ListForSpool(ctx context.Context, spoolID string) ([]domain.DryingSession, error) {
	return s.dryings.List(ctx, spoolID)
}

func (s *DryingService) GetRecommendation(ctx context.Context, material domain.Material) *domain.DryingProfile {
	if s.settings != nil {
		profiles, err := s.settings.ListDryingProfiles(ctx)
		if err == nil {
			for _, p := range profiles {
				if p.Material == material {
					return &p
				}
			}
		}
	}
	// Fallback to defaults
	if p, ok := domain.DefaultDryingProfiles[material]; ok {
		return &p
	}
	return nil
}
