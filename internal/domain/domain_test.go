package domain

import (
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

func TestDryingSchedule(t *testing.T) {
	now := time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)
	profile := DryingProfile{MaxDaysNoDry: 7}
	last := now.AddDate(0, 0, -3)

	if got := NextDryingAt(last, profile); !got.Equal(now.AddDate(0, 0, 4)) {
		t.Fatalf("NextDryingAt() = %v, want %v", got, now.AddDate(0, 0, 4))
	}
	if got := DaysUntilNextDrying(last, profile, now); got != 4 {
		t.Fatalf("DaysUntilNextDrying() = %d, want 4", got)
	}
	if got := DaysUntilNextDrying(now.AddDate(0, 0, -9), profile, now); got != -2 {
		t.Fatalf("overdue DaysUntilNextDrying() = %d, want -2", got)
	}
	if got := DaysUntilNextDrying(time.Time{}, profile, now); got != 0 {
		t.Fatalf("never-dried DaysUntilNextDrying() = %d, want 0", got)
	}
}

func TestMaterialDefinition(t *testing.T) {
	material := MaterialDefinition{
		Name: MaterialPLA, DensityGCM3: 1.24, DryingTemperatureC: 45,
		DryingDurationHours: 4, MaxDaysNoDry: 30,
	}
	if err := material.Validate(); err != nil {
		t.Fatalf("valid material definition: %v", err)
	}
	if profile := material.DryingProfile(); profile.Material != MaterialPLA || profile.TemperatureC != 45 {
		t.Fatalf("drying profile = %+v", profile)
	}
}

func TestNewSpoolDisplayCode(t *testing.T) {
	spool := &Spool{
		Manufacturer: "Bambu Lab",
		Material:     MaterialPLA,
		Color:        "Black",
	}

	got := NewSpoolDisplayCode(spool, "a7f3")
	if got != "PLA-BLA-BAM-A7F3" {
		t.Fatalf("display code = %q", got)
	}

	got = NewSpoolDisplayCode(&Spool{Material: "CF-PETG"}, "spool-42")
	if got != "CFP-COL-MFR-OL42" {
		t.Fatalf("fallback display code = %q", got)
	}
}

func TestCancelledPrinterEventUsage(t *testing.T) {
	spool := Spool{DiameterMM: 1.75, DensityGCM3: 1.24}
	for _, progress := range []int{0, 25} {
		event := PrinterPrintEvent{
			Status: PrintStatusCancelled, CompletionPercent: progress,
			EstimatedWeightG: 40, EstimatedLengthMM: 10000, ActualLengthMM: 100,
		}
		job := CreatePrinterJobFromPrinterEvent(Printer{}, event, spool)
		if job.CompletionPercent != progress || job.ActualWeightG != WeightFromLength(100, 1.75, 1.24) || job.EstimatedLengthMM != 100 {
			t.Fatalf("cancelled job = %+v", job)
		}
	}
	job := CreatePrinterJobFromPrinterEvent(Printer{}, PrinterPrintEvent{
		Status: PrintStatusCancelled, EstimatedWeightG: 40,
	}, spool)
	if job.CompletionPercent != 0 || job.ActualWeightG != 0 {
		t.Fatalf("cancelled before extrusion = %+v", job)
	}
}

func TestCancelledHistoryWithoutEstimateDoesNotReportComplete(t *testing.T) {
	job := CreatePrinterJobFromPrinterHistoryItem(PrinterHistoryJob{
		Status: PrinterHistoryStatusCancelled, FilamentUsedMM: 100,
	}, &Spool{DiameterMM: 1.75, DensityGCM3: 1.24}, PrintStatusCancelled)
	if job.CompletionPercent != 0 || job.ActualWeightG <= 0 || job.EstimatedWeightG != 0 {
		t.Fatalf("cancelled history = %+v", job)
	}
}
