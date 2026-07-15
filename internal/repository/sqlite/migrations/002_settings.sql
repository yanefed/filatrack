-- +goose Up
CREATE TABLE custom_materials (
    name TEXT PRIMARY KEY
);

CREATE TABLE drying_profiles (
    material       TEXT PRIMARY KEY,
    temperature_c  INTEGER NOT NULL,
    duration_hours REAL NOT NULL,
    max_days_no_dry INTEGER NOT NULL
);

CREATE TABLE app_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed default drying profiles
INSERT INTO drying_profiles (material, temperature_c, duration_hours, max_days_no_dry) VALUES
    ('PLA', 45, 4, 30),
    ('PETG', 65, 4, 14),
    ('ABS', 60, 4, 30),
    ('ASA', 60, 4, 30),
    ('TPU', 50, 5, 7),
    ('Nylon', 70, 6, 7),
    ('PC', 70, 8, 7);

-- Default language
INSERT INTO app_settings (key, value) VALUES ('language', 'en');

-- +goose Down
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS drying_profiles;
DROP TABLE IF EXISTS custom_materials;
