package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
)

func (s *printerHistory) syncHistoryPages(ctx context.Context, printer domain.Printer, client port.PrinterHistoryPageClient, importJobs func([]domain.PrinterHistoryJob) (int, error)) (int, error) {
	const pageLimit = 100
	key := "printer_history_sync_v2:" + printer.ID
	now := time.Now()
	checkpoint := historyCheckpoint{URL: printer.MoonrakerURL}
	incremental, supportsIncremental := client.(port.IncrementalHistoryClient)
	if supportsIncremental && s.settings != nil {
		if raw, err := s.settings.GetSetting(ctx, key); err == nil {
			var saved historyCheckpoint
			if json.Unmarshal([]byte(raw), &saved) == nil && saved.URL == printer.MoonrakerURL {
				checkpoint = saved
			}
		}
	}
	// Reconcile old records daily, including missing metadata and history resets.
	full := checkpoint.FullSync.IsZero() || now.Sub(checkpoint.FullSync) >= 24*time.Hour || now.Before(checkpoint.FullSync)
	since := checkpoint.Since
	if full {
		since = time.Time{}
	}
	newest := since
	if !newest.IsZero() {
		newest = newest.Add(time.Second)
	}
	var unfinished time.Time
	unknownStart := false
	created := 0
	for start := 0; ; {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		var jobs []domain.PrinterHistoryJob
		var total int
		var err error
		if supportsIncremental {
			jobs, total, err = incremental.ListHistoryPageSince(ctx, printer, start, pageLimit, since)
		} else {
			jobs, total, err = client.ListHistoryPage(ctx, printer, start, pageLimit)
		}
		if err != nil {
			return created, err
		}
		if len(jobs) == 0 {
			break
		}
		for _, job := range jobs {
			if job.StartedAt.IsZero() {
				// Without a start time a safe incremental boundary is unknown.
				unknownStart = true
				continue
			}
			if job.StartedAt.After(newest) {
				newest = job.StartedAt
			}
			if _, finished := job.PrintStatus(); !finished {
				if unfinished.IsZero() || job.StartedAt.Before(unfinished) {
					unfinished = job.StartedAt
				}
			}
		}
		n, err := importJobs(jobs)
		created += n
		if err != nil {
			return created, err
		}
		start += len(jobs)
		// Some Moonraker versions report the page size as count, not the total.
		if total > len(jobs) && start >= total || total <= len(jobs) && len(jobs) < pageLimit {
			break
		}
	}
	if !unfinished.IsZero() {
		newest = unfinished
	}
	checkpoint.Since = time.Time{}
	if !unknownStart && !newest.IsZero() {
		// Moonraker's since filter is exclusive; overlap preserves equal timestamps.
		checkpoint.Since = newest.Add(-time.Second)
	}
	if full {
		checkpoint.FullSync = now
	}
	if supportsIncremental && s.settings != nil {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		raw, err := json.Marshal(checkpoint)
		if err != nil {
			return created, err
		}
		if err := s.settings.SetSetting(ctx, key, string(raw)); err != nil {
			return created, fmt.Errorf("saving printer history checkpoint: %w", err)
		}
	}
	return created, nil
}
