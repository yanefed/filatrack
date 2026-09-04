package domain

import (
	"fmt"
	"slices"
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

var BuiltInMaterials = []Material{MaterialPLA, MaterialPETG, MaterialABS, MaterialASA, MaterialTPU, MaterialNylon, MaterialPC}

type DryingProfile struct {
	Material      Material
	TemperatureC  int
	DurationHours float64
	MaxDaysNoDry  int
}

type CustomMaterial struct {
	Name                  Material
	DryingProfileMaterial Material
}

var DefaultDryingProfiles = map[Material]DryingProfile{
	MaterialPLA:   {Material: MaterialPLA, TemperatureC: 45, DurationHours: 4, MaxDaysNoDry: 30},
	MaterialPETG:  {Material: MaterialPETG, TemperatureC: 65, DurationHours: 4, MaxDaysNoDry: 14},
	MaterialABS:   {Material: MaterialABS, TemperatureC: 60, DurationHours: 4, MaxDaysNoDry: 30},
	MaterialASA:   {Material: MaterialASA, TemperatureC: 60, DurationHours: 4, MaxDaysNoDry: 30},
	MaterialTPU:   {Material: MaterialTPU, TemperatureC: 50, DurationHours: 5, MaxDaysNoDry: 7},
	MaterialNylon: {Material: MaterialNylon, TemperatureC: 70, DurationHours: 6, MaxDaysNoDry: 7},
	MaterialPC:    {Material: MaterialPC, TemperatureC: 70, DurationHours: 8, MaxDaysNoDry: 7},
}

var DefaultDensities = map[Material]float64{
	MaterialPLA:   1.24,
	MaterialPETG:  1.27,
	MaterialABS:   1.04,
	MaterialASA:   1.07,
	MaterialTPU:   1.21,
	MaterialNylon: 1.14,
	MaterialPC:    1.20,
}

func (m Material) IsBuiltIn() bool {
	return slices.Contains(BuiltInMaterials, m)
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
