-- +goose Up
INSERT INTO app_settings (key, value) VALUES ('currency', 'RUB');
ALTER TABLE print_jobs ADD COLUMN usage_measured INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE print_jobs DROP COLUMN usage_measured;
DELETE FROM app_settings WHERE key = 'currency';
