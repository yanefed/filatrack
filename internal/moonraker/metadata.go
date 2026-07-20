package moonraker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"awesomeProject/internal/domain"
)

type FileMetadata struct {
	FilamentTotalMM     float64
	FilamentWeightTotal float64
	EstimatedTime       time.Duration
	LayerHeight         float64
	NozzleTempC         int
	BedTempC            int
	Slicer              string
}

type MetadataFetcher interface {
	Fetch(ctx context.Context, printer domain.Printer, filename string) (FileMetadata, error)
}

type ConnectionChecker interface {
	Check(ctx context.Context, printer domain.Printer) error
}

type HTTPMetadataFetcher struct {
	Client *http.Client
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return FileMetadata{}, fmt.Errorf("decoding moonraker metadata: %w", err)
	}

	return FileMetadata{
		FilamentTotalMM:     decoded.Result.FilamentTotal,
		FilamentWeightTotal: decoded.Result.FilamentWeightTotal,
		EstimatedTime:       secondsDuration(decoded.Result.EstimatedTime),
		LayerHeight:         decoded.Result.LayerHeight,
		NozzleTempC:         int(decoded.Result.FirstLayerExtrTemp),
		BedTempC:            int(decoded.Result.FirstLayerBedTemp),
		Slicer:              decoded.Result.Slicer,
	}, nil
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
