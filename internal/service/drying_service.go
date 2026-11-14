package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

type DryingService struct {
	dryings  port.DryingSessionRepository
	settings port.SettingsRepository
}

func NewDryingService(dryings port.DryingSessionRepository, settings port.SettingsRepository) *DryingService {
	return &DryingService{dryings: dryings, settings: settings}
}

func (s *DryingService) Create(ctx context.Context, session *domain.DryingSession) error {
	if session == nil {
		return fmt.Errorf("drying session is required")
	}
	if session.TemperatureC <= 0 {
		return fmt.Errorf("temperature must be positive")
	}
	if session.DurationHours <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	now := time.Now()
	if session.DriedAt.IsZero() {
		session.DriedAt = now
	}
	session.CreatedAt = now
	return s.dryings.Create(ctx, session)
}

func (s *DryingService) ListForSpool(ctx context.Context, spoolID string) ([]domain.DryingSession, error) {
	return s.dryings.List(ctx, spoolID)
}

func (s *DryingService) GetRecommendation(ctx context.Context, material domain.Material) (*domain.DryingProfile, error) {
	if s.settings == nil {
		return nil, nil
	}
	profile, err := s.settings.GetDryingProfileForMaterial(ctx, material)
	if err != nil {
		return nil, fmt.Errorf("getting drying profile: %w", err)
	}
	return profile, nil
}

func (s *DryingService) ListRecommendations(ctx context.Context) ([]domain.DryingProfile, error) {
	if s.settings == nil {
		return nil, nil
	}
	materials, err := s.settings.ListMaterials(ctx)
	if err != nil {
		return nil, err
	}
	profiles := make([]domain.DryingProfile, len(materials))
	for i := range materials {
		profiles[i] = materials[i].DryingProfile()
	}
	return profiles, nil
}

// GetStatus возвращает последнюю сушку и рассчитывает срок следующей по
// актуальному профилю материала.
func (s *DryingService) GetStatus(ctx context.Context, spool *domain.Spool) (DryingStatus, error) {
	if spool == nil {
		return DryingStatus{}, fmt.Errorf("spool is required")
	}
	profile, err := s.GetRecommendation(ctx, spool.Material)
	if err != nil {
		return DryingStatus{}, err
	}
	status := DryingStatus{Profile: profile}
	last, err := s.dryings.GetLastDrying(ctx, spool.ID)
	if err != nil {
		return DryingStatus{}, err
	}
	status.LastSession = last
	return calculateDryingStatus(status, time.Now()), nil
}
