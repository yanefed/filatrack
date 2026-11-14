package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
)

func TestLanguageChangesOnlyAfterPersistence(t *testing.T) {
	original := i18n.GetLanguage()
	t.Cleanup(func() { i18n.SetLanguage(original) })
	i18n.SetLanguage(i18n.LangEN)
	wantErr := errors.New("write failed")
	repo := &settingsRepoStub{setSettingErr: wantErr}
	svc := NewSettingsService(repo)
	if err := svc.SetLanguage(t.Context(), i18n.LangBG); !errors.Is(err, wantErr) {
		t.Fatal(err)
	}
	if i18n.GetLanguage() != i18n.LangEN {
		t.Fatal("failed persistence changed active language")
	}
	repo.setSettingErr = nil
	if err := svc.SetLanguage(t.Context(), i18n.LangBG); err != nil {
		t.Fatal(err)
	}
	if i18n.GetLanguage() != i18n.LangBG {
		t.Fatal("successful save did not apply language")
	}
}

func TestDryingStatusPropagatesProfileFailure(t *testing.T) {
	wantErr := errors.New("profile unavailable")
	settings := &settingsRepoStub{profilesErr: wantErr}
	dryings := &dryingRepoStub{}
	spool := &domain.Spool{ID: "spool", Material: domain.MaterialPLA}
	if _, err := NewDryingService(dryings, settings).GetStatus(t.Context(), spool); !errors.Is(err, wantErr) {
		t.Fatalf("status: %v", err)
	}
	if _, err := NewSpoolService(&spoolRepoStub{}, dryings, settings).NeedsDrying(t.Context(), spool); !errors.Is(err, wantErr) {
		t.Fatalf("needs drying: %v", err)
	}
}

func TestCancelledModelRefreshDoesNotReplaceCache(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "one printer", true: "all printers"}[all], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			printer := domain.Printer{ID: "printer", Enabled: true, Kind: domain.PrinterKindMoonraker}
			cache := &printerModelRepoStub{list: []domain.PrinterModelFile{{Path: "cached.gcode"}}}
			client := &printerJobClientStub{
				models: []domain.PrinterModelFile{{Path: "new.gcode"}},
				fetchMetadata: func(context.Context, domain.Printer, string) (*domain.GcodeMetadata, error) {
					cancel()
					return &domain.GcodeMetadata{FilamentWeightG: 10}, nil
				},
			}
			svc := newTestPrinterService(&printerRepoStub{printer: &printer, list: []domain.Printer{printer}}, &spoolRepoStub{}, &printRepoStub{}, PrinterOptions{JobClient: client, Models: cache})
			var err error
			if all {
				_, err = svc.RefreshAllModels(ctx)
			} else {
				_, err = svc.RefreshModels(ctx, printer.ID)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("refresh: %v", err)
			}
			if cache.savedPrinterID != "" {
				t.Fatal("cancelled refresh replaced cached catalog")
			}
		})
	}
}

func TestLaunchRejectsMissingMetadataAndSettingsFailure(t *testing.T) {
	wantErr := errors.New("settings unavailable")
	for _, missingMetadata := range []bool{false, true} {
		t.Run(map[bool]string{false: "settings failure", true: "missing metadata"}[missingMetadata], func(t *testing.T) {
			client := &printerJobClientStub{metadata: &domain.GcodeMetadata{FilamentWeightG: 10}}
			if missingMetadata {
				client.metadata = nil
			}
			svc := NewPrinterService(&printerRepoStub{printer: &domain.Printer{ID: "p", ActiveSpoolID: "s"}}, &spoolRepoStub{spool: &domain.Spool{TotalWeightG: 100}}, &printRepoStub{}, &settingsRepoStub{getSettingErr: wantErr}, PrinterOptions{JobClient: client})
			_, err := svc.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "p", GcodeFilename: "file.gcode"})
			if err == nil {
				t.Fatal("expected error")
			}
			if !missingMetadata && !errors.Is(err, wantErr) {
				t.Fatalf("settings error lost: %v", err)
			}
			if client.startedFile != "" {
				t.Fatal("started print after failed validation")
			}
		})
	}
}

func TestServiceRejectsMissingInputs(t *testing.T) {
	printer := newTestPrinterService(&printerRepoStub{}, &spoolRepoStub{}, &printRepoStub{})
	spools := NewSpoolService(&spoolRepoStub{}, &dryingRepoStub{}, &settingsRepoStub{})
	checks := []func() error{
		func() error { return printer.CreateMoonrakerPrinter(t.Context(), nil) },
		func() error { return printer.UpdateMoonrakerPrinter(t.Context(), nil) },
		func() error { return printer.CheckMoonrakerConnection(t.Context(), nil) },
		func() error { return printer.UpdateStatus(t.Context(), nil) },
		func() error { return spools.Create(t.Context(), nil) },
		func() error { return spools.Update(t.Context(), nil) },
		func() error { return NewDryingService(&dryingRepoStub{}, nil).Create(t.Context(), nil) },
		func() error { return NewSettingsService(&settingsRepoStub{}).UpdateMaterial(t.Context(), nil) },
	}
	for i, check := range checks {
		if err := check(); err == nil {
			t.Fatalf("check %d accepted missing input", i)
		}
	}
}

func TestHistoryUsageWithoutSpoolDoesNotWriteUnchangedData(t *testing.T) {
	prints := &printRepoStub{}
	svc := newTestPrinterService(&printerRepoStub{}, &spoolRepoStub{}, prints)
	local := domain.PrintJob{ID: "job", Status: domain.PrintStatusCancelled, EstimatedLengthMM: 100}
	item := domain.PrinterHistoryJob{Status: domain.PrinterHistoryStatusCancelled, FilamentUsedMM: 100}
	if err := svc.updateExistingHistoryUsage(t.Context(), local, item); err != nil {
		t.Fatal(err)
	}
	if prints.usageJobID != "" {
		t.Fatal("unchanged usage was written again")
	}
}

func TestDryingUsesSameTimestampForNewSession(t *testing.T) {
	session := &domain.DryingSession{TemperatureC: 45, DurationHours: 4}
	if err := NewDryingService(&dryingRepoStub{}, nil).Create(t.Context(), session); err != nil {
		t.Fatal(err)
	}
	if session.DriedAt.IsZero() || !session.DriedAt.Equal(session.CreatedAt) {
		t.Fatal("session timestamps differ")
	}
	// A supplied drying timestamp must survive creation.
	session.DriedAt = time.Unix(100, 0)
	if err := NewDryingService(&dryingRepoStub{}, nil).Create(t.Context(), session); err != nil {
		t.Fatal(err)
	}
	if !session.DriedAt.Equal(time.Unix(100, 0)) {
		t.Fatal("provided drying timestamp changed")
	}
}
