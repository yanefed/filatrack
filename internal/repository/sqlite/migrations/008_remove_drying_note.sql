-- +goose Up
ALTER TABLE drying_sessions
    DROP COLUMN notes;

-- +goose Down
ALTER TABLE drying_sessions
    ADD COLUMN
        notes TEXT NOT NULL DEFAULT '';
