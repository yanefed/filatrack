package sqlite

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

func TestArchiveDetachUndoAndConcurrentChanges(t *testing.T) {
	db := setupTestDB(t)
	spools, printers := NewSpoolRepo(db), NewPrinterRepo(db)
	ctx := t.Context()
	spool := testSpool("original", time.Now().UTC())
	other := testSpool("other", time.Now().UTC())
	for _, s := range []*domain.Spool{spool, other} {
		if err := spools.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"a", "b"} {
		if err := printers.Create(ctx, &domain.Printer{ID: id, Name: id, Kind: domain.PrinterKindMoonraker, MoonrakerURL: "http://printer", State: domain.PrinterStateReady, ActiveSpoolID: spool.ID}); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := spools.ArchiveWithAssignments(ctx, spool.ID, nil)
	if !errors.Is(err, domain.ErrSpoolAssigned) || len(receipt.Printers) != 2 {
		t.Fatalf("confirmation: %+v, %v", receipt, err)
	}
	b, _ := printers.GetByID(ctx, "b")
	b.State = domain.PrinterStatePaused
	if err := printers.UpdateStatus(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := spools.ArchiveWithAssignments(ctx, spool.ID, []string{"a", "b"}); !errors.Is(err, domain.ErrSpoolInUse) {
		t.Fatalf("paused: %v", err)
	}
	original, _ := spools.GetByID(ctx, spool.ID)
	a, _ := printers.GetByID(ctx, "a")
	if original.Archived || a.ActiveSpoolID != spool.ID {
		t.Fatal("blocked archive changed data")
	}
	b.State = domain.PrinterStateReady
	if err := printers.UpdateStatus(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := spools.ArchiveWithAssignments(ctx, spool.ID, []string{"a"}); !errors.Is(err, domain.ErrSpoolAssigned) {
		t.Fatalf("detached an unconfirmed printer: %v", err)
	}
	receipt, err = spools.ArchiveWithAssignments(ctx, spool.ID, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := spools.Update(ctx, original); !errors.Is(err, domain.ErrArchiveChanged) {
		t.Fatalf("stale form must not unarchive: %v", err)
	}
	if err := printers.AssignSpool(ctx, "a", spool.ID); !errors.Is(err, domain.ErrArchiveChanged) {
		t.Fatalf("assigned archived spool: %v", err)
	}
	if err := printers.AssignSpool(ctx, "a", other.ID); err != nil {
		t.Fatal(err)
	}
	// Editing stale printer settings must not restore the old assignment or state.
	a.Name = "edited"
	if err := printers.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	restored, err := spools.Restore(ctx, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.SkippedPrinterIDs) != 1 || restored.SkippedPrinterIDs[0] != "a" {
		t.Fatalf("conflicts: %+v", restored)
	}
	a, _ = printers.GetByID(ctx, "a")
	b, _ = printers.GetByID(ctx, "b")
	if a.ActiveSpoolID != other.ID || b.ActiveSpoolID != spool.ID {
		t.Fatalf("undo assignments: %+v %+v", a, b)
	}
	if _, err := spools.Restore(ctx, receipt); !errors.Is(err, domain.ErrArchiveChanged) {
		t.Fatalf("repeated undo: %v", err)
	}
}

func TestArchiveRollsBackDetachOnFailure(t *testing.T) {
	db := setupTestDB(t)
	ctx := t.Context()
	spools, printers := NewSpoolRepo(db), NewPrinterRepo(db)
	spool := testSpool("spool", time.Now())
	if err := spools.Create(ctx, spool); err != nil {
		t.Fatal(err)
	}
	if err := printers.Create(ctx, &domain.Printer{ID: "p", Name: "p", Kind: domain.PrinterKindMoonraker, MoonrakerURL: "http://p", State: domain.PrinterStateReady, ActiveSpoolID: spool.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.conn.Exec(`CREATE TRIGGER fail_archive BEFORE UPDATE ON spools WHEN NEW.archived=1 BEGIN SELECT RAISE(ABORT, 'archive failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := spools.ArchiveWithAssignments(ctx, spool.ID, []string{"p"}); err == nil {
		t.Fatal("expected failure")
	}
	p, _ := printers.GetByID(ctx, "p")
	s, _ := spools.GetByID(ctx, spool.ID)
	if p.ActiveSpoolID != spool.ID || s.Archived {
		t.Fatal("partial archive committed")
	}
}

func TestHistoryUnicodeSearchAndStableCursor(t *testing.T) {
	db := setupTestDB(t)
	ctx := t.Context()
	repo := NewPrintJobRepo(db)
	when := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	for i := range 305 {
		name := "ordinary"
		if i == 1 || i == 304 {
			name = "КАТУШКА 100%_тест"
		}
		if err := repo.Create(ctx, &domain.PrintJob{ID: fmt.Sprintf("%04d", i), ModelName: name, GcodeFilename: "file.gcode", Status: domain.PrintStatusSuccess, PrintedAt: when, CreatedAt: when}); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := repo.List(ctx, domain.PrintJobFilter{Search: "катушка 100%_", Limit: 100})
	if err != nil || len(matches) != 2 {
		t.Fatalf("unicode/literal search: %d, %v", len(matches), err)
	}
	var cursor *domain.PrintJobCursor
	seen := map[string]bool{}
	for {
		page, err := repo.List(ctx, domain.PrintJobFilter{Before: cursor, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, job := range page {
			if seen[job.ID] {
				t.Fatalf("duplicate %s", job.ID)
			}
			seen[job.ID] = true
		}
		last := page[len(page)-1]
		cursor = &domain.PrintJobCursor{PrintedAt: last.PrintedAt, ID: last.ID}
	}
	if len(seen) != 305 {
		t.Fatalf("loaded %d jobs", len(seen))
	}
	boundary := &domain.PrintJobCursor{PrintedAt: when, ID: "0205"}
	if err := repo.Create(ctx, &domain.PrintJob{ID: "new", ModelName: "new", Status: domain.PrintStatusSuccess, PrintedAt: when.Add(time.Minute), CreatedAt: when}); err != nil {
		t.Fatal(err)
	}
	retained, err := repo.List(ctx, domain.PrintJobFilter{Through: boundary})
	if err != nil || len(retained) != 101 || retained[len(retained)-1].ID != "0205" {
		t.Fatalf("refresh lost the loaded boundary: %d, %v", len(retained), err)
	}

}
