package moonraker

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/preview"

	api "github.com/yanefed/moonraker"
)

type FileMetadata struct {
	FilamentTotalMM     float64
	FilamentWeightTotal float64
	EstimatedTime       time.Duration
	LayerHeight         float64
	NozzleTempC         int
	BedTempC            int
	Slicer              string
	PreviewImagePath    string
	PreviewImageHash    string
	PreviewImageData    []byte
	Thumbnails          []FileThumbnail
}

type FileThumbnail = api.FileThumbnail

type MetadataFetcher interface {
	Fetch(ctx context.Context, printer domain.Printer, filename string) (FileMetadata, error)
}

type ConnectionChecker = port.ConnectionChecker

type HTTPMetadataFetcher struct {
	Client   *http.Client
	CacheDir string
}

var _ port.ConnectionChecker = HTTPMetadataFetcher{}

func (f HTTPMetadataFetcher) Check(ctx context.Context, printer domain.Printer) error {
	return (api.Client{HTTPClient: f.Client}).Check(ctx, printerConfig(printer))
}

func (f HTTPMetadataFetcher) Fetch(ctx context.Context, printer domain.Printer, filename string) (FileMetadata, error) {
	remote, err := (api.Client{HTTPClient: f.Client}).FetchMetadata(ctx, printerConfig(printer), filename)
	if err != nil {
		return FileMetadata{}, err
	}
	metadata := FileMetadata{
		FilamentTotalMM: remote.FilamentTotalMM, FilamentWeightTotal: remote.FilamentWeightTotal,
		EstimatedTime: remote.EstimatedTime, LayerHeight: remote.LayerHeight,
		NozzleTempC: remote.NozzleTempC, BedTempC: remote.BedTempC, Slicer: remote.Slicer,
		PreviewImageData: remote.ThumbnailData, Thumbnails: remote.Thumbnails,
	}
	if len(metadata.PreviewImageData) > 0 {
		metadata.PreviewImageHash = preview.Hash(metadata.PreviewImageData)
		if f.CacheDir != "" {
			path, err := preview.WriteCache(f.CacheDir, metadata.PreviewImageHash, metadata.PreviewImageData)
			if err != nil {
				// The downloaded bytes remain usable even if the local cache is unavailable.
				slog.Warn("writing metadata preview cache failed", "filename", filename, "error", err)
			} else {
				metadata.PreviewImagePath = path
			}
		}
	}
	return metadata, nil
}
