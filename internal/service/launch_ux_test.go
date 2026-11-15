package service

import (
	"errors"
	"testing"

	"github.com/yanefed/filatrack/internal/domain"
)

func TestLaunchPreviewAndConfirmationDoNotAssignSpool(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "p", ActiveSpoolID: "old"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "selected", TotalWeightG: 60}}
	client := &printerJobClientStub{metadata: &domain.GcodeMetadata{FilamentWeightG: 20}}
	svc := newTestPrinterService(printers, spools, &printRepoStub{}, PrinterOptions{JobClient: client})
	req := domain.PrintLaunchRequest{PrinterID: "p", SpoolID: "selected", GcodeFilename: "file.gcode"}
	preview, err := svc.PreviewLaunch(t.Context(), req)
	if err != nil || !preview.RequiresUserConfirmation || printers.assignedID != "" || client.startedFile != "" {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	result, err := svc.StartPrinterFile(t.Context(), req)
	if err != nil || !result.RequiresUserConfirmation || printers.assignedID != "" {
		t.Fatalf("warning assigned a spool: %+v, %v", result, err)
	}
	// Conditions are checked again even when the user confirmed the warning.
	req.AllowLowFilament = true
	spools.used = 50
	result, err = svc.StartPrinterFile(t.Context(), req)
	if err != nil || !result.Filament.Blocked || printers.assignedID != "" {
		t.Fatalf("confirmation bypassed insufficient filament: %+v, %v", result, err)
	}
	spools.used = 0
	result, err = svc.StartPrinterFile(t.Context(), req)
	if err != nil || !result.Started || !result.SpoolAssigned || !result.LaunchAttempted || printers.assignedSpool != "selected" {
		t.Fatalf("confirmed launch: %+v, %v", result, err)
	}
}

func TestLaunchFailureReportsAssignmentAndCommandStage(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "p"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "s", TotalWeightG: 100}}
	client := &printerJobClientStub{metadata: &domain.GcodeMetadata{FilamentWeightG: 10}, startErr: errors.New("connection lost")}
	svc := newTestPrinterService(printers, spools, &printRepoStub{}, PrinterOptions{JobClient: client})
	result, err := svc.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "p", SpoolID: "s", GcodeFilename: "file"})
	if !errors.Is(err, client.startErr) || result == nil || !result.SpoolAssigned || !result.LaunchAttempted || result.Started {
		t.Fatalf("ambiguous remote failure: %+v, %v", result, err)
	}
	printers.printer.ActiveSpoolID = ""
	printers.assignErr = errors.New("database unavailable")
	result, err = svc.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "p", SpoolID: "s", GcodeFilename: "file"})
	if !errors.Is(err, printers.assignErr) || result == nil || result.SpoolAssigned || result.LaunchAttempted {
		t.Fatalf("assignment failure: %+v, %v", result, err)
	}
}
