package shared

import (
	"context"
	"fmt"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"github.com/yanefed/filatrack/internal/service"
)

func ActiveSpools(ctx context.Context, spoolSvc *service.SpoolService) []domain.Spool {
	notArchived := false
	spools, err := spoolSvc.List(ctx, domain.SpoolFilter{Archived: &notArchived})
	if err != nil {
		return nil
	}
	return spools
}

// AllSpools возвращает все катушки, включая архивированные — для отображения
// назначенных катушек в истории печатей и на карточках принтеров.
func AllSpools(ctx context.Context, spoolSvc *service.SpoolService) []domain.Spool {
	spools, err := spoolSvc.List(ctx, domain.SpoolFilter{})
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
