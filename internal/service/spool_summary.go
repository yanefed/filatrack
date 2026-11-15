package service

import (
	"context"
	"errors"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

type SpoolSummary struct {
	Spool      domain.Spool
	RemainingG float64
	Drying     DryingStatus
	DryingErr  error
}

// ListSummaries needs four repository reads regardless of the number of cards.
func (s *SpoolService) ListSummaries(ctx context.Context, filter domain.SpoolFilter) ([]SpoolSummary, error) {
	spools, err := s.spools.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(spools) == 0 {
		return nil, nil
	}
	weights, err := s.spools.GetUsedWeights(ctx)
	if err != nil {
		return nil, err
	}
	last, lastErr := s.drying.dryings.Latest(ctx)
	profiles := make(map[domain.Material]domain.DryingProfile)
	var profileErr error
	if s.settings != nil {
		materials, err := s.settings.ListMaterials(ctx)
		profileErr = err
		if err == nil {
			for _, material := range materials {
				profiles[material.Name] = material.DryingProfile()
			}
		}
	}
	dryingErr := errors.Join(lastErr, profileErr)
	now := time.Now()
	summaries := make([]SpoolSummary, len(spools))
	for i, spool := range spools {
		status := DryingStatus{}
		if profile, ok := profiles[spool.Material]; ok {
			status.Profile = &profile
		}
		if lastErr == nil {
			if session, ok := last[spool.ID]; ok {
				status.LastSession = &session
			}
		}
		if dryingErr == nil {
			status = calculateDryingStatus(status, now)
		} else {
			status = DryingStatus{}
		}
		summaries[i] = SpoolSummary{
			Spool: spool, RemainingG: spool.RemainingWeightG(weights[spool.ID]),
			Drying: status, DryingErr: dryingErr,
		}
	}
	return summaries, nil
}

func calculateDryingStatus(status DryingStatus, now time.Time) DryingStatus {
	if status.Profile == nil {
		return status
	}
	if status.LastSession == nil {
		status.NeedsDrying = true
		return status
	}
	last := status.LastSession.DriedAt
	status.NextDryingAt = domain.NextDryingAt(last, *status.Profile)
	status.DaysUntilNext = domain.DaysUntilNextDrying(last, *status.Profile, now)
	status.NeedsDrying = domain.NeedsDryingAt(last, *status.Profile, now)
	return status
}

func (s *DryingService) ListHistory(ctx context.Context) ([]domain.DryingSession, error) {
	return s.dryings.ListAll(ctx)
}
