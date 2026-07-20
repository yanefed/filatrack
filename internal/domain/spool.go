package domain

import "time"

type Spool struct {
	ID           string
	DisplayCode  string
	Manufacturer string
	Material     Material
	Color        string
	ColorHex     string
	DiameterMM   float64
	TotalWeightG float64
	Price        float64
	DensityGCM3  float64
	PurchasedAt  time.Time
	Archived     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s *Spool) RemainingWeightG(usedG float64) float64 {
	return max(s.TotalWeightG-usedG, 0)
}

func (s *Spool) RemainingPercent(usedG float64) float64 {
	if s.TotalWeightG == 0 {
		return 0
	}
	return s.RemainingWeightG(usedG) / s.TotalWeightG * 100
}
