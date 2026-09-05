package moonraker

import (
	"testing"
	"time"

	"awesomeProject/internal/domain"
)

func TestProcessorRecordsCompletedPrint(t *testing.T) {
	processor := NewProcessor("printer-1")
	started := time.Unix(1000, 0)

	status, event := processor.Apply(StatusUpdate{
		State:         domain.PrinterStatePrinting,
		RawPrintState: "printing",
		Filename:      "benchy.gcode",
		Progress:      0.5,
	}, started)
	if status.State != domain.PrinterStatePrinting || event != nil {
		t.Fatalf("printing update = (%+v, %+v)", status, event)
	}

	finished := started.Add(time.Hour)
	_, event = processor.Apply(StatusUpdate{
		State:          domain.PrinterStateReady,
		RawPrintState:  "complete",
		Filename:       "benchy.gcode",
		Progress:       1,
		FilamentUsedMM: 7654,
		PrintDuration:  time.Hour,
	}, finished)
	if event == nil {
		t.Fatal("expected completed print event")
	}
	if event.PrinterID != "printer-1" || event.GcodeFilename != "benchy.gcode" || event.Status != domain.PrintStatusSuccess || event.CompletionPercent != 100 || event.ActualLengthMM != 7654 || event.PrintDuration != time.Hour || event.PrintedAt != finished {
		t.Fatalf("completed event = %+v", event)
	}
	if event.ExternalPrintID != "printer-1:benchy.gcode:1000" {
		t.Fatalf("external print id = %q", event.ExternalPrintID)
	}
}

func TestProcessorRecordsCancelledPrintWithProgress(t *testing.T) {
	processor := NewProcessor("printer-1")
	now := time.Unix(1000, 0)
	processor.Apply(StatusUpdate{RawPrintState: "printing", Filename: "part.gcode", Progress: 0.42}, now)

	_, event := processor.Apply(StatusUpdate{RawPrintState: "cancelled", Filename: "part.gcode"}, now.Add(time.Minute))
	if event == nil || event.Status != domain.PrintStatusCancelled || event.CompletionPercent != 42 {
		t.Fatalf("cancelled event = %+v", event)
	}
}

func TestProcessorPreservesStateAcrossPartialUpdates(t *testing.T) {
	processor := NewProcessor("printer-1")
	now := time.Unix(1000, 0)

	status, event := processor.Apply(StatusUpdate{
		RawPrintState: "printing",
		Filename:      "part.gcode",
		Progress:      0.1,
	}, now)
	if event != nil || status.State != domain.PrinterStatePrinting || status.CurrentFile != "part.gcode" || status.Progress != 0.1 {
		t.Fatalf("initial update = (%+v, %+v)", status, event)
	}

	status, event = processor.Apply(StatusUpdate{
		Progress:    0.35,
		HasProgress: true,
	}, now.Add(time.Second))
	if event != nil || status.State != domain.PrinterStatePrinting || status.CurrentFile != "part.gcode" || status.Progress != 0.35 {
		t.Fatalf("partial progress update = (%+v, %+v)", status, event)
	}
}

func TestProcessorPreservesFilamentUsedUntilFinalUpdate(t *testing.T) {
	processor := NewProcessor("printer-1")
	now := time.Unix(1000, 0)

	processor.Apply(StatusUpdate{
		RawPrintState:  "printing",
		Filename:       "part.gcode",
		FilamentUsedMM: 1234,
		PrintDuration:  time.Minute,
	}, now)

	_, event := processor.Apply(StatusUpdate{
		RawPrintState: "complete",
	}, now.Add(time.Hour))
	if event == nil || event.ActualLengthMM != 1234 || event.PrintDuration != time.Minute {
		t.Fatalf("final event = %+v", event)
	}
}

func TestProcessorCancelledWithZeroProgress(t *testing.T) {
	processor := NewProcessor("printer-1")
	now := time.Unix(1000, 0)
	processor.Apply(StatusUpdate{RawPrintState: "printing", Filename: "part.gcode"}, now)
	_, event := processor.Apply(StatusUpdate{RawPrintState: "cancelled", HasProgress: true}, now.Add(time.Minute))
	if event == nil || event.CompletionPercent != 0 {
		t.Fatalf("cancelled event = %+v", event)
	}
}

func TestProcessorCancelledAfterProgressReset(t *testing.T) {
	processor := NewProcessor("printer-1")
	now := time.Unix(1000, 0)
	processor.Apply(StatusUpdate{RawPrintState: "printing", Filename: "part.gcode", Progress: 0.42, FilamentUsedMM: 100}, now)
	_, event := processor.Apply(StatusUpdate{RawPrintState: "cancelled", HasProgress: true}, now.Add(time.Minute))
	if event == nil || event.CompletionPercent != 42 || event.ActualLengthMM != 100 {
		t.Fatalf("cancelled event = %+v", event)
	}
}
