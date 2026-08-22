package domain

import (
	"math"
	"time"
)

type DryingSession struct {
	ID            string
	SpoolID       string
	TemperatureC  int
	DurationHours float64
	DriedAt       time.Time
	CreatedAt     time.Time
}

func NeedsDrying(lastDried time.Time, profile DryingProfile) bool {
	if lastDried.IsZero() {
		return true
	}
	daysSinceDry := time.Since(lastDried).Hours() / 24
	return daysSinceDry > float64(profile.MaxDaysNoDry)
}

// NextDryingAt возвращает дату, к которой катушку нужно высушить снова.
// Для катушки без истории сушки возвращается нулевое время.
func NextDryingAt(lastDried time.Time, profile DryingProfile) time.Time {
	if lastDried.IsZero() {
		return time.Time{}
	}
	return lastDried.AddDate(0, 0, profile.MaxDaysNoDry)
}

// DaysUntilNextDrying возвращает число полных отображаемых дней до следующей
// сушки. Отрицательное значение означает просрочку.
func DaysUntilNextDrying(lastDried time.Time, profile DryingProfile, now time.Time) int {
	dueAt := NextDryingAt(lastDried, profile)
	if dueAt.IsZero() {
		return 0
	}
	days := dueAt.Sub(now).Hours() / 24
	if days >= 0 {
		return int(math.Ceil(days))
	}
	return -int(math.Ceil(-days))
}
