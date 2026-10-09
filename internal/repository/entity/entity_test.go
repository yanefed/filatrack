package entity

import (
	"reflect"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

func TestSpoolRoundTrip(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	want := domain.Spool{
		ID: "spool-1", DisplayCode: "PLA-BLA-ACM-0001", Manufacturer: "ACME",
		Material: domain.MaterialPLA, Color: "Black", ColorHex: "#000000",
		DiameterMM: 1.75, TotalWeightG: 1000, Price: 25, DensityGCM3: 1.24,
		PurchasedAt: now, Archived: true, CreatedAt: now, UpdatedAt: now,
	}
	persisted := SpoolFromDomain(want)
	if !persisted.PurchasedAt.Valid || persisted.Material != "PLA" || persisted.Archived != 1 {
		t.Fatalf("persistence spool = %+v", persisted)
	}
	if got := persisted.Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestPrintJobRoundTrip(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	want := domain.PrintJob{
		ID: "job-1", ExternalPrintID: "remote-1", GcodeFilename: "cube.gcode",
		ModelName: "cube", Status: domain.PrintStatusSuccess, CompletionPercent: 100,
		EstimatedWeightG: 10, ActualWeightG: 10, EstimatedLengthMM: 3000,
		UsageMeasured: true,
		PrintDuration: 90 * time.Second, NozzleTempC: 210, BedTempC: 60,
		Slicer: "OrcaSlicer", PreviewImageHash: "preview-hash",
		PreviewImagePath: "/tmp/cube.png", PreviewImageData: []byte("preview"),
		PrintedAt: now, CreatedAt: now,
	}
	persisted := PrintJobFromDomain(want)
	if persisted.SpoolID.Valid || persisted.PrinterID.Valid || persisted.PrintDurationSec != 90 || !persisted.PreviewImageHash.Valid || persisted.PreviewImagePath != "" {
		t.Fatalf("persistence print job = %+v", persisted)
	}
	// Cache paths and bytes are reconstructed by the service from the stored hash.
	want.PreviewImagePath = ""
	want.PreviewImageData = nil
	if got := persisted.Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestPrinterRoundTrip(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	want := domain.Printer{
		ID: "printer-1", Name: "Voron", Kind: domain.PrinterKindMoonraker,
		MoonrakerURL: "http://printer.local", Enabled: true,
		State: domain.PrinterStateReady, Progress: 0.5,
		LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	persisted := PrinterFromDomain(want)
	if persisted.Enabled != 1 || persisted.ActiveSpoolID.Valid || !persisted.LastSeenAt.Valid {
		t.Fatalf("persistence printer = %+v", persisted)
	}
	if got := persisted.Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestDryingSessionRoundTrip(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	want := domain.DryingSession{
		ID: "dry-1", SpoolID: "spool-1", TemperatureC: 45,
		DurationHours: 4, DriedAt: now, CreatedAt: now,
	}
	if got := DryingSessionFromDomain(want).Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestPrinterModelRoundTrip(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	want := domain.PrinterModelFile{
		PrinterID: "printer-1", Path: "models/cube.gcode", Filename: "cube.gcode",
		ModifiedAt: now, SizeBytes: 1024, EstimatedWeightG: 12,
		PrintDuration: time.Hour, Slicer: "PrusaSlicer", CachedAt: now,
		PreviewImageHash: "preview-hash", PreviewImagePath: "/tmp/cube.png", PreviewImageData: []byte("preview"),
	}
	persisted := PrinterModelFromDomain(want)
	if !persisted.ModifiedAt.Valid || persisted.PrintDurationSec != 3600 || !persisted.PreviewImageHash.Valid || persisted.PreviewImagePath != "" {
		t.Fatalf("persistence printer model = %+v", persisted)
	}
	want.PreviewImagePath = ""
	want.PreviewImageData = nil
	if got := persisted.Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestMaterialRoundTrip(t *testing.T) {
	want := domain.MaterialDefinition{
		Name: domain.MaterialPETG, DensityGCM3: 1.27, DryingTemperatureC: 65,
		DryingDurationHours: 4, MaxDaysNoDry: 14,
	}
	if got := MaterialFromDomain(want).Domain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}
