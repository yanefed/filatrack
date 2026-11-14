package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/service"
)

// Unused remote operations are supplied by the embedded interface and must never run.
type modelCacheClient struct {
	port.PrinterJobClient
	requests int
}

func (c *modelCacheClient) ListModels(_ context.Context, p domain.Printer) ([]domain.PrinterModelFile, error) {
	return []domain.PrinterModelFile{{PrinterID: p.ID, Path: "part.gcode", ModifiedAt: time.Unix(100, 0), SizeBytes: 100}}, nil
}
func (c *modelCacheClient) FetchMetadata(context.Context, domain.Printer, string) (*domain.GcodeMetadata, error) {
	c.requests++
	return &domain.GcodeMetadata{FilamentLengthMM: 1000}, nil
}

func TestIncrementalModelCacheSurvivesDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.db")
	app := openApplication(t, path)
	printer := domain.Printer{Name: "Printer", MoonrakerURL: "http://printer"}
	if err := app.printers.CreateMoonrakerPrinter(t.Context(), &printer); err != nil {
		t.Fatal(err)
	}
	client := &modelCacheClient{}
	options := service.PrinterOptions{JobClient: client, Models: sqlite.NewPrinterModelRepo(app.db)}
	svc := newPrinterService(app.db, options)
	models, err := svc.RefreshModels(t.Context(), printer.ID)
	if err != nil || len(models) != 1 || !models[0].MetadataReady || models[0].FilamentLengthMM != 1000 {
		t.Fatalf("refresh: %+v %v", models, err)
	}
	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}
	app = openApplication(t, path)
	t.Cleanup(func() {
		if err := app.db.Close(); err != nil {
			t.Error(err)
		}
	})
	options.Models = sqlite.NewPrinterModelRepo(app.db)
	svc = newPrinterService(app.db, options)
	if _, err := svc.RefreshModels(t.Context(), printer.ID); err != nil {
		t.Fatal(err)
	}
	if client.requests != 1 {
		t.Fatalf("metadata fetched again after reopening DB: %d", client.requests)
	}
}
