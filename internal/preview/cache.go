package preview

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteCache stores an image by its content hash and returns its local path.
func WriteCache(cacheDir, hash string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("preview image is empty")
	}
	if hash == "" || Hash(data) != hash {
		return "", fmt.Errorf("preview image hash does not match its data")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("creating preview cache: %w", err)
	}
	path := filepath.Join(cacheDir, hash)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return path, nil
	}

	tmp, err := os.CreateTemp(cacheDir, ".preview-*")
	if err != nil {
		return "", fmt.Errorf("creating preview cache file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("writing preview cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("closing preview cache file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("installing preview cache file: %w", err)
	}
	return path, nil
}
