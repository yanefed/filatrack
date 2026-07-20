package domain

import (
	"math"
	"testing"
	"time"
)

func TestSpoolRemainingWeight(t *testing.T) {
	spool := &Spool{TotalWeightG: 1000}
	tests := []struct {
		name string
		used float64
		want float64
	}{
		{name: "unused", used: 0, want: 1000},
		{name: "partially used", used: 250, want: 750},
		{name: "overused is clamped", used: 1200, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spool.RemainingWeightG(tt.used); got != tt.want {
				t.Fatalf("RemainingWeightG(%v) = %v, want %v", tt.used, got, tt.want)
			}
		})
	}
}

func TestSpoolRemainingPercent(t *testing.T) {
	if got := (&Spool{}).RemainingPercent(10); got != 0 {
		t.Fatalf("zero-weight spool percentage = %v, want 0", got)
	}
	if got := (&Spool{TotalWeightG: 800}).RemainingPercent(200); got != 75 {
		t.Fatalf("remaining percentage = %v, want 75", got)
	}
}

func TestPrintJobCalculateActualWeight(t *testing.T) {
	job := &PrintJob{EstimatedWeightG: 40, CompletionPercent: 25}
	if got := job.CalculateActualWeight(); got != 10 {
		t.Fatalf("actual weight = %v, want 10", got)
	}
}

func TestNeedsDrying(t *testing.T) {
	profile := DryingProfile{MaxDaysNoDry: 7}
	tests := []struct {
		name string
		last time.Time
		want bool
	}{
		{name: "never dried", want: true},
		{name: "recently dried", last: time.Now().Add(-6 * 24 * time.Hour), want: false},
		{name: "overdue", last: time.Now().Add(-8 * 24 * time.Hour), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsDrying(tt.last, profile); got != tt.want {
				t.Fatalf("NeedsDrying() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultMaterialData(t *testing.T) {
	materials := []Material{MaterialPLA, MaterialPETG, MaterialABS, MaterialASA, MaterialTPU, MaterialNylon, MaterialPC}
	for _, material := range materials {
		profile, ok := DefaultDryingProfiles[material]
		if !ok || profile.Material != material || profile.TemperatureC <= 0 || profile.DurationHours <= 0 || profile.MaxDaysNoDry <= 0 {
			t.Errorf("invalid drying profile for %s: %+v", material, profile)
		}
		if density := DefaultDensities[material]; density <= 0 || math.IsNaN(density) {
			t.Errorf("invalid density for %s: %v", material, density)
		}
	}
}
