-- +goose Up
INSERT INTO app_settings (key, value)
VALUES ('low_filament_level', '50')
ON CONFLICT(key) DO NOTHING;

-- +goose Down
DELETE
FROM app_settings
WHERE key = 'low_filament_level';
