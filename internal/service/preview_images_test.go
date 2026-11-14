package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yanefed/filatrack/internal/preview"
)

type memoryPreviewImages struct {
	data []byte
}

func (m *memoryPreviewImages) Put(_ context.Context, data []byte) (string, error) {
	m.data = append([]byte(nil), data...)
	return preview.Hash(data), nil
}

func (m *memoryPreviewImages) Get(context.Context, string) ([]byte, error) {
	return append([]byte(nil), m.data...), nil
}

func TestImportLegacyStoresImageInSharedRepositoryAndCache(t *testing.T) {
	legacyPath := filepath.Join(t.TempDir(), "old-preview.png")
	want := []byte("legacy preview bytes")
	if err := os.WriteFile(legacyPath, want, 0o600); err != nil {
		t.Fatal(err)
	}
	images := &memoryPreviewImages{}
	store := NewPreviewImageStore(images, t.TempDir())

	hash, path, err := store.ImportLegacy(context.Background(), legacyPath)
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if hash != preview.Hash(want) {
		t.Fatalf("hash = %q, want %q", hash, preview.Hash(want))
	}
	if string(images.data) != string(want) {
		t.Fatalf("stored bytes = %q, want %q", images.data, want)
	}
	cached, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading cache %q: %v", path, err)
	}
	if string(cached) != string(want) {
		t.Fatalf("cached bytes = %q, want %q", cached, want)
	}
}

func TestImportLegacyResolvesRelativePathFromDataDirectory(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := filepath.Join("cache", "gcode-previews", "old.png")
	want := []byte("legacy preview bytes")
	fullPath := filepath.Join(dataDir, legacyPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, want, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewPreviewImageStore(&memoryPreviewImages{}, filepath.Join(dataDir, "cache", "gcode-previews"))

	hash, path, err := store.ImportLegacy(context.Background(), legacyPath)
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if hash != preview.Hash(want) || path == "" {
		t.Fatalf("ImportLegacy() = (%q, %q), want hash and cached path", hash, path)
	}
}

func TestImportLegacySkipsMissingPath(t *testing.T) {
	store := NewPreviewImageStore(&memoryPreviewImages{}, t.TempDir())
	hash, path, err := store.ImportLegacy(context.Background(), "cache/gcode-previews/missing.png")
	if err != nil || hash != "" || path != "" {
		t.Fatalf("ImportLegacy() = (%q, %q, %v), want empty result without error", hash, path, err)
	}
}
