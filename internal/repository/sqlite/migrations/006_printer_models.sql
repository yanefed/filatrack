-- +goose Up
CREATE TABLE printer_models (
    printer_id          TEXT NOT NULL REFERENCES printers(id) ON DELETE CASCADE,
    path                TEXT NOT NULL,
    filename            TEXT NOT NULL DEFAULT '',
    modified_at         DATETIME,
    size_bytes          INTEGER NOT NULL DEFAULT 0,
    estimated_weight_g  REAL NOT NULL DEFAULT 0,
    print_duration_sec  INTEGER NOT NULL DEFAULT 0,
    slicer              TEXT NOT NULL DEFAULT '',
    preview_image_path  TEXT NOT NULL DEFAULT '',
    cached_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (printer_id, path)
);

CREATE INDEX idx_printer_models_cached_at ON printer_models(cached_at);

-- +goose Down
DROP INDEX IF EXISTS idx_printer_models_cached_at;
DROP TABLE IF EXISTS printer_models;
