package domain

import (
	"fmt"
)

type Material string

const (
	MaterialPLA   Material = "PLA"
	MaterialPETG  Material = "PETG"
	MaterialABS   Material = "ABS"
	MaterialASA   Material = "ASA"
	MaterialTPU   Material = "TPU"
	MaterialNylon Material = "Nylon"
	MaterialPC    Material = "PC"
)

const DefaultDensity float64 = 1.24
const DefaultDiameter float64 = 1.75

type MaterialDefinition struct {
	Name                Material
	DensityGCM3         float64
	DryingTemperatureC  int
	DryingDurationHours float64
	MaxDaysNoDry        int
}

func (m MaterialDefinition) DryingProfile() DryingProfile {
	return DryingProfile{
		Material:      m.Name,
		TemperatureC:  m.DryingTemperatureC,
		DurationHours: m.DryingDurationHours,
		MaxDaysNoDry:  m.MaxDaysNoDry,
	}
}

func (m *MaterialDefinition) Validate() error {
	if m == nil {
		return fmt.Errorf("material is required")
	}
	if m.Name == "" {
		return fmt.Errorf("material name is required")
	}
	if m.DensityGCM3 <= 0 {
		return fmt.Errorf("material density must be positive")
	}
	if m.DryingTemperatureC <= 0 {
		return fmt.Errorf("drying temperature must be positive")
	}
	if m.DryingDurationHours <= 0 {
		return fmt.Errorf("drying duration must be positive")
	}
	if m.MaxDaysNoDry <= 0 {
		return fmt.Errorf("max days without drying must be positive")
	}
	return nil
}

type DryingProfile struct {
	Material      Material
	TemperatureC  int
	DurationHours float64
	MaxDaysNoDry  int
}

func (p *DryingProfile) ValidateDryingProfile() error {
	if p == nil {
		return fmt.Errorf("drying profile is required")
	}
	if p.Material == "" {
		return fmt.Errorf("material is required")
	}
	if p.TemperatureC <= 0 {
		return fmt.Errorf("temperature must be positive")
	}
	if p.DurationHours <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if p.MaxDaysNoDry <= 0 {
		return fmt.Errorf("max days must be positive")
	}
	return nil
}
