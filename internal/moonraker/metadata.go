package moonraker

import (
	"awesomeProject/internal/domain"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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
	Thumbnails          []FileThumbnail
}

type FileThumbnail struct {
	Width        int
	Height       int
	Size         int64
	RelativePath string
}

type MetadataFetcher interface {
	Fetch(ctx context.Context, printer domain.Printer, filename string) (FileMetadata, error)
}

type ConnectionChecker interface {
	Check(ctx context.Context, printer domain.Printer) error
}

type HTTPMetadataFetcher struct {
	Client   *http.Client
	CacheDir string
}

func (f HTTPMetadataFetcher) Check(ctx context.Context, printer domain.Printer) error {
	endpoint, err := serverInfoURL(printer.MoonrakerURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating server info request: %w", err)
	}
	if printer.APIKey != "" {
		req.Header.Set("X-Api-Key", printer.APIKey)
	}

	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("requesting moonraker server info: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("moonraker server info returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (f HTTPMetadataFetcher) Fetch(ctx context.Context, printer domain.Printer, filename string) (FileMetadata, error) {
	endpoint, err := metadataURL(printer.MoonrakerURL, filename)
	if err != nil {
		return FileMetadata{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("creating metadata request: %w", err)
	}
	if printer.APIKey != "" {
		req.Header.Set("X-Api-Key", printer.APIKey)
	}

	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("requesting moonraker metadata: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FileMetadata{}, fmt.Errorf("moonraker metadata returned HTTP %d", resp.StatusCode)
	}

	var decoded struct {
		Result struct {
			FilamentTotal       float64 `json:"filament_total"`
			FilamentWeightTotal float64 `json:"filament_weight_total"`
			EstimatedTime       float64 `json:"estimated_time"`
			LayerHeight         float64 `json:"layer_height"`
			FirstLayerExtrTemp  float64 `json:"first_layer_extr_temp"`
			FirstLayerBedTemp   float64 `json:"first_layer_bed_temp"`
			Slicer              string  `json:"slicer"`
			Thumbnails          []struct {
				Width        int    `json:"width"`
				Height       int    `json:"height"`
				Size         int64  `json:"size"`
				RelativePath string `json:"relative_path"`
			} `json:"thumbnails"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return FileMetadata{}, fmt.Errorf("decoding moonraker metadata: %w", err)
	}

	metadata := FileMetadata{
		FilamentTotalMM:     decoded.Result.FilamentTotal,
		FilamentWeightTotal: decoded.Result.FilamentWeightTotal,
		EstimatedTime:       secondsDuration(decoded.Result.EstimatedTime),
		LayerHeight:         decoded.Result.LayerHeight,
		NozzleTempC:         int(decoded.Result.FirstLayerExtrTemp),
		BedTempC:            int(decoded.Result.FirstLayerBedTemp),
		Slicer:              decoded.Result.Slicer,
	}
	for _, thumbnail := range decoded.Result.Thumbnails {
		if thumbnail.RelativePath == "" {
			continue
		}
		metadata.Thumbnails = append(metadata.Thumbnails, FileThumbnail{
			Width:        thumbnail.Width,
			Height:       thumbnail.Height,
			Size:         thumbnail.Size,
			RelativePath: thumbnail.RelativePath,
		})
	}
	if f.CacheDir != "" {
		metadata.PreviewImagePath = f.cacheBestThumbnail(ctx, client, printer, filename, metadata.Thumbnails)
	}
	return metadata, nil
}

func (f HTTPMetadataFetcher) cacheBestThumbnail(ctx context.Context, client *http.Client, printer domain.Printer, filename string, thumbnails []FileThumbnail) string {
	thumbnail, ok := bestThumbnail(thumbnails)
	if !ok {
		return ""
	}
	endpoint, err := thumbnailURL(printer.MoonrakerURL, filename, thumbnail.RelativePath)
	if err != nil {
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	if printer.APIKey != "" {
		req.Header.Set("X-Api-Key", printer.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	if err := os.MkdirAll(f.CacheDir, 0o755); err != nil {
		return ""
	}
	cachePath := filepath.Join(f.CacheDir, cachedThumbnailFilename(printer, filename, thumbnail.RelativePath))
	file, err := os.Create(cachePath)
	if err != nil {
		return ""
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(cachePath)
		return ""
	}
	return cachePath
}

func bestThumbnail(thumbnails []FileThumbnail) (FileThumbnail, bool) {
	if len(thumbnails) == 0 {
		return FileThumbnail{}, false
	}
	best := thumbnails[0]
	bestArea := best.Width * best.Height
	for _, thumbnail := range thumbnails[1:] {
		area := thumbnail.Width * thumbnail.Height
		if area > bestArea || area == bestArea && thumbnail.Size > best.Size {
			best = thumbnail
			bestArea = area
		}
	}
	return best, true
}

func metadataURL(rawMoonrakerURL, filename string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/server/files/metadata")
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("filename", filename)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func serverInfoURL(rawMoonrakerURL string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/server/info")
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func thumbnailURL(rawMoonrakerURL, gcodeFilename, relativePath string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "")
	if err != nil {
		return "", err
	}
	thumbnailPath := strings.TrimPrefix(relativePath, "/")
	if dir := path.Dir(gcodeFilename); dir != "." && dir != "/" {
		thumbnailPath = path.Join(dir, thumbnailPath)
	}
	parsed.Path = path.Join("/server/files/gcodes", thumbnailPath)
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func cachedThumbnailFilename(printer domain.Printer, gcodeFilename, relativePath string) string {
	sum := sha1.Sum([]byte(printer.ID + "|" + printer.MoonrakerURL + "|" + gcodeFilename + "|" + relativePath))
	ext := strings.ToLower(filepath.Ext(relativePath))
	if ext == "" || len(ext) > 8 {
		ext = ".png"
	}
	return hex.EncodeToString(sum[:]) + ext
}

func moonrakerHTTPURL(rawMoonrakerURL, path string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawMoonrakerURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("valid moonraker URL is required")
	}
	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	case "http", "https":
	default:
		return nil, fmt.Errorf("moonraker URL must use http, https, ws, or wss")
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}
