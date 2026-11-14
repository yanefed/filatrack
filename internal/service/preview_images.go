package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/port"
	"github.com/yanefed/filatrack/internal/preview"
)

type PreviewImageStore struct {
	images   port.PreviewImageRepository
	cacheDir string
}

func NewPreviewImageStore(images port.PreviewImageRepository, cacheDir string) *PreviewImageStore {
	return &PreviewImageStore{images: images, cacheDir: cacheDir}
}

func (s *PreviewImageStore) Store(ctx context.Context, data []byte) (string, string, error) {
	if len(data) == 0 {
		return "", "", nil
	}
	if s == nil || s.images == nil {
		return preview.Hash(data), "", nil
	}
	hash, err := s.images.Put(ctx, data)
	if err != nil {
		return "", "", err
	}
	path, err := preview.WriteCache(s.cacheDir, hash, data)
	if err != nil {
		slog.Warn("writing local preview cache failed", "hash", hash, "error", err)
		return hash, "", nil
	}
	return hash, path, nil
}

// ImportLegacy copies an image referenced by an old local path into shared storage.
func (s *PreviewImageStore) ImportLegacy(ctx context.Context, legacyPath string) (string, string, error) {
	if legacyPath == "" {
		return "", "", nil
	}
	data, err := s.readLegacy(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("reading legacy preview image: %w", err)
	}
	return s.Store(ctx, data)
}

func (s *PreviewImageStore) readLegacy(legacyPath string) ([]byte, error) {
	candidates := []string{legacyPath}
	if !filepath.IsAbs(legacyPath) && s != nil && s.cacheDir != "" {
		dataDir := filepath.Clean(filepath.Join(s.cacheDir, "..", ".."))
		candidates = append(candidates, filepath.Join(dataDir, legacyPath))
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, os.ErrNotExist
}

func (s *PreviewImageStore) Resolve(ctx context.Context, hash, legacyPath string) (string, error) {
	if s == nil || s.images == nil {
		return legacyPath, nil
	}
	if hash == "" {
		if legacyPath != "" {
			if _, err := os.Stat(legacyPath); err == nil {
				return legacyPath, nil
			}
		}
		return "", nil
	}
	path := filepath.Join(s.cacheDir, hash)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	data, err := s.images.Get(ctx, hash)
	if err != nil {
		return "", err
	}
	path, err = preview.WriteCache(s.cacheDir, hash, data)
	if err != nil {
		slog.Warn("restoring local preview cache failed", "hash", hash, "error", err)
		return "", nil
	}
	return path, nil
}

func (s *PreviewImageStore) HydrateModels(ctx context.Context, models []domain.PrinterModelFile) error {
	for i := range models {
		path, err := s.Resolve(ctx, models[i].PreviewImageHash, models[i].PreviewImagePath)
		if err != nil {
			return err
		}
		models[i].PreviewImagePath = path
	}
	return nil
}

func (s *PreviewImageStore) HydratePrintJobs(ctx context.Context, jobs []domain.PrintJob) error {
	for i := range jobs {
		path, err := s.Resolve(ctx, jobs[i].PreviewImageHash, jobs[i].PreviewImagePath)
		if err != nil {
			return err
		}
		jobs[i].PreviewImagePath = path
	}
	return nil
}

func migrateLegacyPrintPreviews(ctx context.Context, previews *PreviewImageStore, prints port.PrintJobRepository, jobs []domain.PrintJob) error {
	if previews == nil {
		return nil
	}

	for i := range jobs {
		if jobs[i].PreviewImageHash != "" || jobs[i].PreviewImagePath == "" {
			continue
		}
		hash, path, err := previews.ImportLegacy(ctx, jobs[i].PreviewImagePath)
		if err != nil {
			return fmt.Errorf("importing legacy print preview image: %w", err)
		}
		if hash == "" {
			continue
		}
		if err := prints.UpdatePreviewImageHash(ctx, jobs[i].ID, hash); err != nil {
			return fmt.Errorf("saving shared print preview reference: %w", err)
		}
		jobs[i].PreviewImageHash = hash
		jobs[i].PreviewImagePath = path
	}
	return nil
}

func preparePrintPreviews(ctx context.Context, previews *PreviewImageStore, prints port.PrintJobRepository, jobs []domain.PrintJob) error {
	if err := migrateLegacyPrintPreviews(ctx, previews, prints, jobs); err != nil {
		return err
	}
	if err := previews.HydratePrintJobs(ctx, jobs); err != nil {
		return fmt.Errorf("loading print preview images: %w", err)
	}
	return nil
}
