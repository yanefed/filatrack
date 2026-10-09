package moonraker

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"

	api "github.com/yanefed/moonraker"
)

type HistoryClient interface {
	port.PrinterJobClient
	port.PrinterHistoryPageClient
}

type HTTPControlClient struct {
	Client   *http.Client
	CacheDir string
}

var (
	_ port.PrinterJobClient         = HTTPControlClient{}
	_ port.PrinterHistoryPageClient = HTTPControlClient{}
	_ port.IncrementalHistoryClient = HTTPControlClient{}
)

func (c HTTPControlClient) client() api.Client { return api.Client{HTTPClient: c.Client} }

func (c HTTPControlClient) ListHistory(ctx context.Context, printer domain.Printer, limit int) ([]domain.PrinterHistoryJob, error) {
	jobs, err := c.client().ListHistory(ctx, printerConfig(printer), limit)
	if err != nil {
		return nil, err
	}
	return domainHistory(printer.ID, jobs), nil
}

func (c HTTPControlClient) ListHistoryPage(ctx context.Context, printer domain.Printer, start, limit int) ([]domain.PrinterHistoryJob, int, error) {
	return c.ListHistoryPageSince(ctx, printer, start, limit, time.Time{})
}

func (c HTTPControlClient) ListHistoryPageSince(ctx context.Context, printer domain.Printer, start, limit int, since time.Time) ([]domain.PrinterHistoryJob, int, error) {
	jobs, count, err := c.client().ListHistoryPageSince(ctx, printerConfig(printer), start, limit, since)
	if err != nil {
		return nil, 0, err
	}
	return domainHistory(printer.ID, jobs), count, nil
}

func domainHistory(printerID string, jobs []api.HistoryJob) []domain.PrinterHistoryJob {
	result := make([]domain.PrinterHistoryJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, domain.PrinterHistoryJob{
			PrinterID: printerID, ExternalPrintID: job.JobID, GcodeFilename: job.Filename,
			ModelName: modelNameFromFilename(job.Filename), Status: domain.PrinterHistoryStatus(job.Status),
			FilamentUsedMM: job.FilamentUsedMM, EstimatedWeightG: job.EstimatedWeightG,
			PrintDuration: job.PrintDuration, TotalDuration: job.TotalDuration,
			NozzleTempC: job.NozzleTempC, BedTempC: job.BedTempC, Slicer: job.Slicer,
			StartedAt: job.StartedAt, EndedAt: job.EndedAt,
		})
	}
	return result
}

func (c HTTPControlClient) ListModels(ctx context.Context, printer domain.Printer) ([]domain.PrinterModelFile, error) {
	files, err := c.client().ListFiles(ctx, printerConfig(printer))
	if err != nil {
		return nil, err
	}
	models := make([]domain.PrinterModelFile, 0, len(files))
	for _, file := range files {
		models = append(models, domain.PrinterModelFile{
			PrinterID: printer.ID, Path: file.Path, Filename: file.Filename,
			ModifiedAt: file.ModifiedAt, SizeBytes: file.SizeBytes,
		})
	}
	return models, nil
}

func (c HTTPControlClient) StartFile(ctx context.Context, printer domain.Printer, filename string) error {
	return c.client().StartFile(ctx, printerConfig(printer), filename)
}

func (c HTTPControlClient) DownloadFile(ctx context.Context, printer domain.Printer, filename, localPath string) error {
	return c.client().DownloadFile(ctx, printerConfig(printer), filename, localPath)
}

func (c HTTPControlClient) Upload(ctx context.Context, printer domain.Printer, localPath, remotePath string, startPrint bool) (domain.PrinterUploadResult, error) {
	result, err := c.client().Upload(ctx, printerConfig(printer), localPath, remotePath, startPrint)
	if err != nil {
		return domain.PrinterUploadResult{}, err
	}
	return domain.PrinterUploadResult{Path: result.Path, PrintStarted: result.PrintStarted, PrintQueued: result.PrintQueued}, nil
}

func (c HTTPControlClient) FetchMetadata(ctx context.Context, printer domain.Printer, filename string) (*domain.GcodeMetadata, error) {
	metadata, err := HTTPMetadataFetcher{Client: c.Client, CacheDir: c.CacheDir}.Fetch(ctx, printer, filename)
	if err != nil {
		return nil, err
	}
	return &domain.GcodeMetadata{
		Filename:         filename,
		ModelName:        modelNameFromFilename(filename),
		Slicer:           metadata.Slicer,
		FilamentLengthMM: metadata.FilamentTotalMM,
		FilamentWeightG:  metadata.FilamentWeightTotal,
		PrintDuration:    metadata.EstimatedTime,
		NozzleTempC:      metadata.NozzleTempC,
		BedTempC:         metadata.BedTempC,
		LayerHeight:      metadata.LayerHeight,
		PreviewImagePath: metadata.PreviewImagePath,
		PreviewImageHash: metadata.PreviewImageHash,
		PreviewImageData: metadata.PreviewImageData,
	}, nil
}

func modelNameFromFilename(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}
