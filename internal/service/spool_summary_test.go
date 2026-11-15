package service

import (
	"errors"
	"testing"

	"github.com/yanefed/filatrack/internal/domain"
)

func TestSummaryDoesNotReportDryingRequiredWhenItsInputsFailed(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, profileFailed := range []bool{false, true} {
		settings := &settingsRepoStub{materials: []domain.MaterialDefinition{{Name: domain.MaterialPLA, MaxDaysNoDry: 30}}}
		dryings := &dryingRepoStub{}
		if profileFailed {
			settings.materialsErr = failure
		} else {
			dryings.lastErr = failure
		}
		svc := NewSpoolService(&spoolRepoStub{list: []domain.Spool{{ID: "s", Material: domain.MaterialPLA, TotalWeightG: 1000}}, used: 100}, dryings, settings)
		items, err := svc.ListSummaries(t.Context(), domain.SpoolFilter{})
		if err != nil || len(items) != 1 {
			t.Fatalf("summaries: %+v %v", items, err)
		}
		if !errors.Is(items[0].DryingErr, failure) || items[0].Drying.NeedsDrying || items[0].RemainingG != 900 {
			t.Fatalf("summary: %+v", items[0])
		}
	}
}
func TestSummaryRejectsFailedWeightRead(t *testing.T) {
	failure := errors.New("weight read failed")
	svc := NewSpoolService(&spoolRepoStub{list: []domain.Spool{{ID: "s"}}, usedErr: failure}, &dryingRepoStub{}, nil)
	if _, err := svc.ListSummaries(t.Context(), domain.SpoolFilter{}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
