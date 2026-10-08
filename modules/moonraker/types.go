// Package moonraker provides HTTP operations and WebSocket status decoding for
// the Moonraker API used by Klipper printers. It has no application dependencies.
package moonraker

import (
	"net/http"
	"time"
)

// Config identifies a Moonraker server. URL accepts http, https, ws, or wss.
// APIKey is optional for servers that allow trusted clients.
type Config struct {
	URL    string
	APIKey string
}

// Client performs HTTP operations. A nil HTTPClient uses http.DefaultClient.
// Use a client timeout or context deadline to bound requests.
type Client struct {
	HTTPClient *http.Client
}

// HistoryJob represents a remote job, without local printer or spool identifiers.
// Status preserves Moonraker's original value, including unknown future states.
type HistoryJob struct {
	JobID            string
	Filename         string
	Status           string
	FilamentUsedMM   float64
	EstimatedWeightG float64
	PrintDuration    time.Duration
	TotalDuration    time.Duration
	NozzleTempC      int
	BedTempC         int
	Slicer           string
	StartedAt        time.Time
	EndedAt          time.Time
}

// File describes a file in Moonraker's gcodes root.
type File struct {
	Path       string
	Filename   string
	ModifiedAt time.Time
	SizeBytes  int64
}

// UploadResult reports Moonraker's response to a G-code upload.
type UploadResult struct {
	Path         string
	PrintStarted bool
	PrintQueued  bool
}
