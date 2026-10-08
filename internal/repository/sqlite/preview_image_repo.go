package sqlite

import (
	"awesomeProject/internal/port"
	"awesomeProject/internal/preview"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PreviewImageRepo struct{ db *DB }

func NewPreviewImageRepo(db *DB) port.PreviewImageRepository {
	return &PreviewImageRepo{db: db}
}

func (r *PreviewImageRepo) Put(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("preview image data is empty")
	}
	hash := preview.Hash(data)
	if _, err := r.db.conn.ExecContext(ctx,
		`INSERT INTO preview_images (hash, image_data) VALUES (?, ?) ON CONFLICT (hash) DO NOTHING`, hash, data,
	); err != nil {
		return "", fmt.Errorf("storing preview image: %w", err)
	}
	return hash, nil
}

func (r *PreviewImageRepo) Get(ctx context.Context, hash string) ([]byte, error) {
	var data []byte
	err := r.db.conn.QueryRowContext(ctx, `SELECT image_data FROM preview_images WHERE hash = ?`, hash).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("preview image not found: %s", hash)
	}
	if err != nil {
		return nil, fmt.Errorf("loading preview image: %w", err)
	}
	return data, nil
}
