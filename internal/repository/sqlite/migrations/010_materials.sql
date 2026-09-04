-- +goose Up
CREATE TABLE materials (
    name                   TEXT PRIMARY KEY,
    density_g_cm3          REAL NOT NULL CHECK (density_g_cm3 > 0),
    drying_temperature_c   INTEGER NOT NULL CHECK (drying_temperature_c > 0),
    drying_duration_hours  REAL NOT NULL CHECK (drying_duration_hours > 0),
    max_days_no_dry        INTEGER NOT NULL CHECK (max_days_no_dry > 0)
);

-- Preserve all existing drying-profile edits and give known materials their
-- physical densities. Profiles that were created for custom materials are
-- migrated as regular materials as well.
INSERT INTO materials (
    name, density_g_cm3, drying_temperature_c,
    drying_duration_hours, max_days_no_dry
)
SELECT
    material,
    CASE material
        WHEN 'PLA' THEN 1.24
        WHEN 'PETG' THEN 1.27
        WHEN 'ABS' THEN 1.04
        WHEN 'ASA' THEN 1.07
        WHEN 'TPU' THEN 1.21
        WHEN 'Nylon' THEN 1.14
        WHEN 'PC' THEN 1.20
        ELSE 1.24
    END,
    temperature_c,
    duration_hours,
    max_days_no_dry
FROM drying_profiles;

-- Migrate custom materials that use another material's drying profile.
INSERT OR IGNORE INTO materials (
    name, density_g_cm3, drying_temperature_c,
    drying_duration_hours, max_days_no_dry
)
SELECT
    custom.name,
    1.24,
    COALESCE(profile.temperature_c, 45),
    COALESCE(profile.duration_hours, 4),
    COALESCE(profile.max_days_no_dry, 30)
FROM custom_materials custom
LEFT JOIN drying_profiles profile
    ON profile.material = COALESCE(custom.drying_profile_material, custom.name);

-- Ensure that every standard material exists even if its legacy drying
-- profile had previously been removed.
INSERT OR IGNORE INTO materials VALUES
    ('PLA',   1.24, 45, 4, 30),
    ('PETG',  1.27, 65, 4, 14),
    ('ABS',   1.04, 60, 4, 30),
    ('ASA',   1.07, 60, 4, 30),
    ('TPU',   1.21, 50, 5, 7),
    ('Nylon', 1.14, 70, 6, 7),
    ('PC',    1.20, 70, 8, 7);

DROP INDEX IF EXISTS idx_custom_materials_drying_profile;
DROP TABLE custom_materials;
DROP TABLE drying_profiles;

-- +goose Down
CREATE TABLE drying_profiles (
    material        TEXT PRIMARY KEY,
    temperature_c   INTEGER NOT NULL,
    duration_hours  REAL NOT NULL,
    max_days_no_dry INTEGER NOT NULL
);

CREATE TABLE custom_materials (
    name                     TEXT PRIMARY KEY,
    drying_profile_material  TEXT
        REFERENCES drying_profiles(material) ON UPDATE CASCADE ON DELETE SET NULL
);

INSERT INTO drying_profiles (material, temperature_c, duration_hours, max_days_no_dry)
SELECT name, drying_temperature_c, drying_duration_hours, max_days_no_dry
FROM materials;

INSERT INTO custom_materials (name, drying_profile_material)
SELECT name, name
FROM materials
WHERE name NOT IN ('PLA', 'PETG', 'ABS', 'ASA', 'TPU', 'Nylon', 'PC');

CREATE INDEX idx_custom_materials_drying_profile
    ON custom_materials(drying_profile_material);

DROP TABLE materials;
