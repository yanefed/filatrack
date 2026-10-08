package sqlite

import (
	"awesomeProject/internal/domain"
	"testing"
	"time"
)

func TestPrintHistoryMatchesRemoteIDAndLiveFingerprint(t *testing.T) {
	db := setupTestDB(t)
	ctx := t.Context()
	printers := NewPrinterRepo(db)
	for _, id := range []string{"p", "other"} {
		if err := printers.Create(ctx, &domain.Printer{ID: id, Name: id, Kind: domain.PrinterKindMoonraker, MoonrakerURL: "http://printer"}); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPrintJobRepo(db)
	minute := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	for _, job := range []domain.PrintJob{
		{ID: "remote", PrinterID: "p", ExternalPrintID: "r1", GcodeFilename: "renamed.gcode", PrintedAt: minute.Add(-time.Hour)},
		{ID: "live", PrinterID: "p", ExternalPrintID: "live-id", GcodeFilename: "part.gcode", PrintedAt: minute.Add(10 * time.Second)},
		{ID: "outside", PrinterID: "p", GcodeFilename: "part.gcode", PrintedAt: minute.Add(time.Minute)},
		{ID: "wrong-file", PrinterID: "p", GcodeFilename: "another.gcode", PrintedAt: minute},
		{ID: "wrong-printer", PrinterID: "other", ExternalPrintID: "r1", GcodeFilename: "part.gcode", PrintedAt: minute},
	} {
		job.Status = domain.PrintStatusSuccess
		if err := repo.Create(ctx, &job); err != nil {
			t.Fatal(err)
		}
	}
	printerID := "p"
	jobs, err := repo.List(ctx, domain.PrintJobFilter{PrinterID: &printerID, HistoryMatches: []domain.PrintHistoryMatch{
		{ExternalPrintID: "r1"},
		{ExternalPrintID: "remote-live", Filename: "part.gcode", EndedAt: minute.Add(40 * time.Second)},
	}})
	if err != nil || len(jobs) != 2 || jobs[0].ID != "live" || jobs[1].ID != "remote" {
		t.Fatalf("matched jobs=%+v error=%v", jobs, err)
	}
	jobs, err = repo.List(ctx, domain.PrintJobFilter{PrinterID: &printerID, HistoryMatches: []domain.PrintHistoryMatch{}})
	if err != nil || len(jobs) != 0 {
		t.Fatalf("empty page matched jobs=%+v error=%v", jobs, err)
	}
}
