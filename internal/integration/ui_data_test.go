package integration_test

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/service"
)

type countedSpools struct {
	port.SpoolRepository
	count *atomic.Int64
}

func (r countedSpools) List(ctx context.Context, f domain.SpoolFilter) ([]domain.Spool, error) {
	r.count.Add(1)
	return r.SpoolRepository.List(ctx, f)
}
func (r countedSpools) GetByID(ctx context.Context, id string) (*domain.Spool, error) {
	r.count.Add(1)
	return r.SpoolRepository.GetByID(ctx, id)
}
func (r countedSpools) GetUsedWeight(ctx context.Context, id string) (float64, error) {
	r.count.Add(1)
	return r.SpoolRepository.GetUsedWeight(ctx, id)
}
func (r countedSpools) GetUsedWeights(ctx context.Context) (map[string]float64, error) {
	r.count.Add(1)
	return r.SpoolRepository.GetUsedWeights(ctx)
}

type countedDryings struct {
	port.DryingSessionRepository
	count *atomic.Int64
}

func (r countedDryings) Latest(ctx context.Context) (map[string]domain.DryingSession, error) {
	r.count.Add(1)
	return r.DryingSessionRepository.Latest(ctx)
}
func (r countedDryings) GetLastDrying(ctx context.Context, id string) (*domain.DryingSession, error) {
	r.count.Add(1)
	return r.DryingSessionRepository.GetLastDrying(ctx, id)
}

type countedSettings struct {
	port.SettingsRepository
	count *atomic.Int64
}

func (r countedSettings) ListMaterials(ctx context.Context) ([]domain.MaterialDefinition, error) {
	r.count.Add(1)
	return r.SettingsRepository.ListMaterials(ctx)
}
func (r countedSettings) GetDryingProfileForMaterial(ctx context.Context, m domain.Material) (*domain.DryingProfile, error) {
	r.count.Add(1)
	return r.SettingsRepository.GetDryingProfileForMaterial(ctx, m)
}

func TestSpoolSummariesMatchIndividualReadsWithConstantQueryCount(t *testing.T) {
	app := openApplication(t, t.TempDir()+"/ui.db")
	defer app.db.Close()
	for _, id := range []string{"one", "two", "archived"} {
		spool := domain.Spool{ID: id, Manufacturer: id, Material: domain.MaterialPLA, Color: "White", TotalWeightG: 1000}
		if err := app.spools.Create(t.Context(), &spool); err != nil {
			t.Fatal(err)
		}
		if id == "archived" {
			if err := app.spools.Archive(t.Context(), spool.ID); err != nil {
				t.Fatal(err)
			}
		}
		if id == "one" {
			if err := app.dryings.Create(t.Context(), &domain.DryingSession{SpoolID: spool.ID, TemperatureC: 45, DurationHours: 4, DriedAt: time.Now().Add(-48 * time.Hour)}); err != nil {
				t.Fatal(err)
			}
		}
		if err := sqlite.NewPrintJobRepo(app.db).Create(t.Context(), &domain.PrintJob{SpoolID: spool.ID, GcodeFilename: "part.gcode", Status: domain.PrintStatusCancelled, EstimatedWeightG: 50, ActualWeightG: 25}); err != nil {
			t.Fatal(err)
		}
	}
	var count atomic.Int64
	svc := service.NewSpoolService(countedSpools{sqlite.NewSpoolRepo(app.db), &count}, countedDryings{sqlite.NewDryingRepo(app.db), &count}, countedSettings{sqlite.NewSettingsRepo(app.db), &count})
	active := false
	summaries, err := svc.ListSummaries(t.Context(), domain.SpoolFilter{Archived: &active})
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 || count.Load() != 4 {
		t.Fatalf("%d cards used %d reads", len(summaries), count.Load())
	}
	for _, item := range summaries {
		_, remaining, err := app.spools.GetWithRemaining(t.Context(), item.Spool.ID)
		if err != nil {
			t.Fatal(err)
		}
		drying, err := app.dryings.GetStatus(t.Context(), &item.Spool)
		if err != nil {
			t.Fatal(err)
		}
		if item.RemainingG != remaining || !reflect.DeepEqual(item.Drying, drying) || item.DryingErr != nil {
			t.Fatalf("changed business rules: %+v vs %+v, remaining %f", item, drying, remaining)
		}
	}
}
