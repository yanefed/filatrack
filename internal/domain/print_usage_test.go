package domain

import (
	"math"
	"testing"
	"time"
)

func TestUsageReassignmentPreservesMeasurementAndCompletion(t *testing.T) {
	spool := Spool{DiameterMM: 1.75, DensityGCM3: 1.24}
	original := PrintUsage{EstimatedWeightG: 100, LengthMM: 10000, CompletionPercent: 40, Measured: true}
	usage := original.WithSpool(spool, PrintStatusCancelled)
	if math.Abs(usage.ActualWeightG-29.8254952531416) > 0.00001 {
		t.Fatalf("actual weight = %f", usage.ActualWeightG)
	}
	if usage.CompletionPercent != 40 || !usage.Measured {
		t.Fatalf("lost completion/provenance: %+v", usage)
	}
	spool.DensityGCM3 *= 2
	reassigned := usage.WithSpool(spool, PrintStatusCancelled)
	if reassigned.ActualWeightG != 2*usage.ActualWeightG {
		t.Fatal("new spool properties were not used")
	}
	if reassigned.WithSpool(spool, PrintStatusCancelled) != reassigned {
		t.Fatal("repeated assignment changed usage")
	}
	original.Measured = false
	if original.WithSpool(spool, PrintStatusSuccess).Measured {
		t.Fatal("derived length became measured")
	}
}

func TestUsageCompletionFromConsumedLength(t *testing.T) {
	spool := Spool{DiameterMM: 1.75, DensityGCM3: 1.24}
	for _, tc := range []struct {
		name       string
		status     PrintStatus
		estimated  float64
		completion int
	}{
		{"successful without estimate", PrintStatusSuccess, 0, 100},
		{"cancelled partial", PrintStatusCancelled, 100, 29},
		{"cancelled exceeds estimate", PrintStatusCancelled, 10, 100},
		{"cancelled without estimate", PrintStatusCancelled, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage := (PrintUsage{EstimatedWeightG: tc.estimated, LengthMM: 10000}).WithSpool(spool, tc.status)
			if usage.CompletionPercent != tc.completion {
				t.Fatalf("completion = %d", usage.CompletionPercent)
			}
			if tc.status == PrintStatusSuccess && usage.EstimatedWeightG != usage.ActualWeightG {
				t.Fatal("successful print estimate was not recovered")
			}
		})
	}
	original := PrintUsage{ActualWeightG: 12, CompletionPercent: 20, Measured: true}
	if original.WithSpool(spool, PrintStatusSuccess) != original {
		t.Fatal("no length: existing usage changed")
	}
}

func TestHistoryUsageReconcilesMeasuredConsumption(t *testing.T) {
	spool := Spool{DiameterMM: 1.75, DensityGCM3: 1.24}
	item := PrinterHistoryJob{Status: PrinterHistoryStatusCancelled, EstimatedWeightG: 100, FilamentUsedMM: 10000}
	usage := (PrintUsage{}).WithHistory(item, &spool, PrintStatusCancelled)
	if usage.CompletionPercent != 29 || !usage.Measured {
		t.Fatalf("cancelled usage = %+v", usage)
	}
	if usage.WithHistory(item, &spool, PrintStatusCancelled) != usage {
		t.Fatal("repeat reconciliation changed usage")
	}
	unassigned := (PrintUsage{}).WithHistory(item, nil, PrintStatusCancelled)
	if unassigned.ActualWeightG != 0 || unassigned.Measured {
		t.Fatalf("unassigned usage = %+v", unassigned)
	}
	item.Status = PrinterHistoryStatusCompleted
	item.FilamentUsedMM = 0
	successful := (PrintUsage{}).WithHistory(item, nil, PrintStatusSuccess)
	if successful.ActualWeightG != 100 || successful.CompletionPercent != 100 || successful.Measured {
		t.Fatalf("estimated successful usage = %+v", successful)
	}
}

func TestDryingStatusAtFixedTime(t *testing.T) {
	last := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := DryingProfile{MaxDaysNoDry: 7}
	due := last.AddDate(0, 0, 7)
	if NeedsDryingAt(last, profile, due) {
		t.Fatal("drying is not overdue at the exact boundary")
	}
	if !NeedsDryingAt(last, profile, due.Add(time.Second)) {
		t.Fatal("drying should be overdue")
	}
	if !NeedsDryingAt(time.Time{}, profile, due) {
		t.Fatal("never dried spool needs drying")
	}
}
