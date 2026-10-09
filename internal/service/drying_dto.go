package service

import (
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

// DryingStatus описывает актуальность сушки конкретной катушки.
type DryingStatus struct {
	Profile       *domain.DryingProfile
	LastSession   *domain.DryingSession
	NextDryingAt  time.Time
	DaysUntilNext int
	NeedsDrying   bool
}
