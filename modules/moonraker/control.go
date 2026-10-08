package moonraker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ListHistory returns recent jobs in descending order. A nonpositive limit fetches all pages.
func (c Client) ListHistory(ctx context.Context, server Config, limit int) ([]HistoryJob, error) {
	if limit > 0 {
		return c.listHistoryPage(ctx, server, 0, limit)
	}

	const pageLimit = 100
	var jobs []HistoryJob
	for start := 0; ; {
		page, total, err := c.ListHistoryPage(ctx, server, start, pageLimit)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, page...)
		if len(page) == 0 {
			return jobs, nil
		}
		start += len(page)
		if total > len(page) && start >= total {
			return jobs, nil
		}
		if total <= len(page) && len(page) < pageLimit {
			return jobs, nil
		}
	}
}

func (c Client) listHistoryPage(ctx context.Context, server Config, start, limit int) ([]HistoryJob, error) {
	jobs, _, err := c.listHistoryPageWithCount(ctx, server, start, limit)
	return jobs, err
}

// ListHistoryPage returns a history page and the count reported by Moonraker.
// Depending on the server version, count can be a page size or a total.
func (c Client) ListHistoryPage(ctx context.Context, server Config, start, limit int) ([]HistoryJob, int, error) {
	return c.listHistoryPageWithCount(ctx, server, start, limit)
}

func (c Client) listHistoryPageWithCount(ctx context.Context, server Config, start, limit int) ([]HistoryJob, int, error) {
	return c.ListHistoryPageSince(ctx, server, start, limit, time.Time{})
}

// ListHistoryPageSince filters by job start time, including unfinished jobs.
func (c Client) ListHistoryPageSince(ctx context.Context, server Config, start, limit int, since time.Time) ([]HistoryJob, int, error) {
	endpoint, err := historyListURL(server.URL, start, limit)
	if err != nil {
		return nil, 0, err
	}
	if !since.IsZero() {
		parsed, _ := url.Parse(endpoint)
		query := parsed.Query()
		query.Set("since", strconv.FormatFloat(float64(since.UnixNano())/1e9, 'f', 9, 64))
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating history request: %w", err)
	}
	setAPIKey(req, server)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("requesting moonraker history: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("moonraker history returned HTTP %d", resp.StatusCode)
	}

	var decoded historyListResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, 0, fmt.Errorf("decoding moonraker history: %w", err)
	}
	historyJobs := decoded.jobs()

	jobs := make([]HistoryJob, 0, len(historyJobs))
	for _, job := range historyJobs {
		jobs = append(jobs, job.decode())
	}
	return jobs, decoded.count(), nil
}

// ListFiles lists files in the gcodes root, excluding directory entries.
func (c Client) ListFiles(ctx context.Context, server Config) ([]File, error) {
	endpoint, err := fileListURL(server.URL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating file list request: %w", err)
	}
	setAPIKey(req, server)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting moonraker file list: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("moonraker file list returned HTTP %d", resp.StatusCode)
	}

	var decoded struct {
		Result []fileListItem `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decoding moonraker file list: %w", err)
	}

	models := make([]File, 0, len(decoded.Result))
	for _, item := range decoded.Result {
		if item.Path == "" || item.Type != "" && item.Type != "file" {
			continue
		}
		models = append(models, item.decode())
	}
	return models, nil
}

// StartFile starts printing a G-code file already present on the server.
func (c Client) StartFile(ctx context.Context, server Config, filename string) error {
	endpoint, err := printStartURL(server.URL, filename)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating print start request: %w", err)
	}
	setAPIKey(req, server)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("starting moonraker print: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("moonraker print start returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// DownloadFile saves a remote G-code file to localPath, overwriting an existing file.
func (c Client) DownloadFile(ctx context.Context, server Config, filename, localPath string) (err error) {
	endpoint, err := downloadURL(server.URL, filename)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating file download request: %w", err)
	}
	setAPIKey(req, server)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("downloading moonraker file: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("moonraker file download returned HTTP %d", resp.StatusCode)
	}

	var file *os.File
	file, err = os.Create(localPath)
	if err != nil {
		return fmt.Errorf("creating local download cache: %w", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing local download cache: %w", closeErr)
		}
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("writing local download cache: %w", err)
	}

	return nil
}

// Upload sends a local G-code file to remotePath in the gcodes root.
// When startPrint is true, Moonraker is asked to start or queue the print.
func (c Client) Upload(ctx context.Context, server Config, localPath, remotePath string, startPrint bool) (uploadResult UploadResult, err error) {
	file, err := os.Open(localPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("opening local gcode: %w", err)
	}
	defer func(file *os.File) {
		closeErr := file.Close()
		if closeErr != nil {
			err = fmt.Errorf("closing file: %w", closeErr)
		}
	}(file)

	if remotePath == "" || path.IsAbs(remotePath) || path.Clean(remotePath) != remotePath || remotePath == "." || strings.HasPrefix(remotePath, "../") {
		return UploadResult{}, fmt.Errorf("invalid remote gcode path: %q", remotePath)
	}
	body, contentType, err := uploadForm(file, remotePath, startPrint)
	if err != nil {
		return UploadResult{}, err
	}

	endpoint, err := uploadURL(server.URL)
	if err != nil {
		return UploadResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	setAPIKey(req, server)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("uploading moonraker file: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UploadResult{}, fmt.Errorf("moonraker upload returned HTTP %d", resp.StatusCode)
	}

	var decoded struct {
		Item struct {
			Path string `json:"path"`
		} `json:"item"`
		PrintStarted bool `json:"print_started"`
		PrintQueued  bool `json:"print_queued"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return UploadResult{}, fmt.Errorf("decoding moonraker upload response: %w", err)
	}
	return UploadResult{Path: decoded.Item.Path, PrintStarted: decoded.PrintStarted, PrintQueued: decoded.PrintQueued}, nil
}

func uploadForm(file io.Reader, remotePath string, startPrint bool) (body *bytes.Buffer, contentType string, err error) {
	body = new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	defer func() {
		if closeErr := writer.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing upload form: %w", closeErr)
		}
	}()
	contentType = writer.FormDataContentType()
	part, err := writer.CreateFormFile("file", path.Base(remotePath))
	if err != nil {
		return nil, "", fmt.Errorf("creating upload form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("writing upload form file: %w", err)
	}
	if err := writer.WriteField("root", "gcodes"); err != nil {
		return nil, "", fmt.Errorf("writing upload root field: %w", err)
	}
	if folder := path.Dir(remotePath); folder != "." {
		if err := writer.WriteField("path", folder); err != nil {
			return nil, "", fmt.Errorf("writing upload path field: %w", err)
		}
	}
	if err := writer.WriteField("print", strconv.FormatBool(startPrint)); err != nil {
		return nil, "", fmt.Errorf("writing upload print field: %w", err)
	}
	return body, contentType, nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

type historyJob struct {
	JobID         string          `json:"job_id"`
	Filename      string          `json:"filename"`
	Status        string          `json:"status"`
	FilamentUsed  historyFloat    `json:"filament_used"`
	PrintDuration float64         `json:"print_duration"`
	TotalDuration float64         `json:"total_duration"`
	StartTime     float64         `json:"start_time"`
	EndTime       *float64        `json:"end_time"`
	Metadata      json.RawMessage `json:"metadata"`
}

type historyListResponse struct {
	Count  int          `json:"count"`
	Jobs   []historyJob `json:"jobs"`
	Result struct {
		Count int          `json:"count"`
		Jobs  []historyJob `json:"jobs"`
	} `json:"result"`
}

func (r historyListResponse) jobs() []historyJob {
	if len(r.Result.Jobs) > 0 {
		return r.Result.Jobs
	}
	return r.Jobs
}

func (r historyListResponse) count() int {
	if r.Result.Count > 0 {
		return r.Result.Count
	}
	return r.Count
}

type fileListItem struct {
	Path     string  `json:"path"`
	Type     string  `json:"type"`
	Size     int64   `json:"size"`
	Modified float64 `json:"modified"`
}

func (i fileListItem) decode() File {
	return File{
		Path:       i.Path,
		Filename:   filepath.Base(i.Path),
		ModifiedAt: unixFloatTime(i.Modified),
		SizeBytes:  i.Size,
	}
}

func (j historyJob) decode() HistoryJob {
	metadata := decodeHistoryMetadata(j.Metadata)
	filamentUsed := j.FilamentUsed.Float64()
	if filamentUsed == 0 && metadata.FilamentUsed > 0 {
		filamentUsed = metadata.FilamentUsed
	}
	job := HistoryJob{
		JobID:            j.JobID,
		Filename:         j.Filename,
		Status:           j.Status,
		FilamentUsedMM:   filamentUsed,
		EstimatedWeightG: metadata.FilamentWeightTotal,
		PrintDuration:    secondsDuration(j.PrintDuration),
		TotalDuration:    secondsDuration(j.TotalDuration),
		NozzleTempC:      int(metadata.FirstLayerExtrTemp),
		BedTempC:         int(metadata.FirstLayerBedTemp),
		Slicer:           metadata.Slicer,
		StartedAt:        unixFloatTime(j.StartTime),
	}
	if j.EndTime != nil {
		job.EndedAt = unixFloatTime(*j.EndTime)
	}
	return job
}

type historyMetadata struct {
	FilamentWeightTotal float64 `json:"filament_weight_total"`
	FilamentUsed        float64 `json:"filament_used"`
	FirstLayerExtrTemp  float64 `json:"first_layer_extr_temp"`
	FirstLayerBedTemp   float64 `json:"first_layer_bed_temp"`
	Slicer              string  `json:"slicer"`
	ObjectHeight        float64 `json:"object_height"`
}

type historyFloat float64

func (f *historyFloat) UnmarshalJSON(data []byte) error {
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*f = historyFloat(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		*f = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}
	*f = historyFloat(parsed)
	return nil
}

func (f *historyFloat) Float64() float64 {
	if f == nil {
		return 0
	}
	return float64(*f)
}

func decodeHistoryMetadata(raw json.RawMessage) historyMetadata {
	if len(raw) == 0 {
		return historyMetadata{}
	}
	var metadata historyMetadata
	_ = json.Unmarshal(raw, &metadata)
	return metadata
}

func historyListURL(rawMoonrakerURL string, start, limit int) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/server/history/list")
	if err != nil {
		return "", err
	}
	values := url.Values{}
	if start > 0 {
		values.Set("start", fmt.Sprintf("%d", start))
	}
	values.Set("limit", fmt.Sprintf("%d", limit))
	values.Set("order", "desc")
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func fileListURL(rawMoonrakerURL string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/server/files/list")
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("root", "gcodes")
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func printStartURL(rawMoonrakerURL, filename string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/printer/print/start")
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("filename", filename)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func uploadURL(rawMoonrakerURL string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "/server/files/upload")
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func downloadURL(rawMoonrakerURL, filename string) (string, error) {
	parsed, err := moonrakerHTTPURL(rawMoonrakerURL, "")
	if err != nil {
		return "", err
	}
	parsed.Path = path.Join("/server/files/gcodes", filename)
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func setAPIKey(req *http.Request, server Config) {
	if server.APIKey != "" {
		req.Header.Set("X-Api-Key", server.APIKey)
	}
}

func unixFloatTime(seconds float64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	whole := int64(seconds)
	nanos := int64((seconds - float64(whole)) * float64(time.Second))
	return time.Unix(whole, nanos)
}
