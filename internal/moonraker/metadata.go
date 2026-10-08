package moonraker

import (
	"context"
	"net/http"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/port"
	"awesomeProject/internal/preview"

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
			metadata.PreviewImagePath, _ = preview.WriteCache(f.CacheDir, metadata.PreviewImageHash, metadata.PreviewImageData)
		}
	}
	return metadata, nil
}
