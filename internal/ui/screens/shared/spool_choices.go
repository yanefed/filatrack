package shared

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/service"
	"context"
	"fmt"
)

func ActiveSpools(ctx context.Context, spoolSvc *service.SpoolService) []domain.Spool {
	notArchived := false
	spools, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &notArchived})
	if err != nil {
		return nil
	}
	return spools
}

func SpoolChoiceNames(t i18n.T, spools []domain.Spool) []string {
	names := []string{t.UnassignedSpool}
	for _, spool := range spools {
		names = append(names, SpoolDisplayName(spool))
	}
	return names
}

func SpoolDisplayName(spool domain.Spool) string {
	return fmt.Sprintf("%s — %s %s (%s)", spool.DisplayCode, spool.Manufacturer, spool.Color, spool.Material)
}

func SelectedSpoolIndex(activeSpoolID string, spools []domain.Spool) uint {
	for i, spool := range spools {
		if spool.ID == activeSpoolID {
			return uint(i + 1)
		}
	}
	return 0
}
