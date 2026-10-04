-- +goose Up
CREATE TABLE preview_images (
    hash TEXT PRIMARY KEY,
    image_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE print_jobs
    ADD COLUMN preview_image_hash TEXT REFERENCES preview_images(hash);
ALTER TABLE printer_models
    ADD COLUMN preview_image_hash TEXT REFERENCES preview_images(hash);

-- +goose Down
ALTER TABLE printer_models DROP COLUMN preview_image_hash;
ALTER TABLE print_jobs DROP COLUMN preview_image_hash;
DROP TABLE preview_images;
