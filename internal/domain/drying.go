package domain

import "time"

type DryingSession struct {
	ID            string
	SpoolID       string
	TemperatureC  int
	DurationHours float64
	Notes         string
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
