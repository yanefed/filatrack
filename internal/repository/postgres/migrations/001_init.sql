-- +goose Up
CREATE TABLE materials (
    name TEXT PRIMARY KEY,
    density_g_cm3 DOUBLE PRECISION NOT NULL CHECK (density_g_cm3 > 0),
    drying_temperature_c INTEGER NOT NULL CHECK (drying_temperature_c > 0),
    drying_duration_hours DOUBLE PRECISION NOT NULL CHECK (drying_duration_hours > 0),
    max_days_no_dry INTEGER NOT NULL CHECK (max_days_no_dry > 0)
);
INSERT INTO materials VALUES
    ('PLA', 1.24, 45, 4, 30), ('PETG', 1.27, 65, 4, 14),
    ('ABS', 1.04, 60, 4, 30), ('ASA', 1.07, 60, 4, 30),
    ('TPU', 1.21, 50, 5, 7), ('Nylon', 1.14, 70, 6, 7),
    ('PC', 1.20, 70, 8, 7);

CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO app_settings VALUES ('language', 'en'), ('low_filament_level', '50'), ('currency', 'RUB');

CREATE TABLE spools (
    id TEXT PRIMARY KEY,
    display_code TEXT NOT NULL UNIQUE DEFAULT '',
    manufacturer TEXT NOT NULL,
    material TEXT NOT NULL,
    color TEXT NOT NULL,
    color_hex TEXT NOT NULL DEFAULT '#000000',
    diameter_mm DOUBLE PRECISION NOT NULL DEFAULT 1.75,
    total_weight_g DOUBLE PRECISION NOT NULL,
    price DOUBLE PRECISION NOT NULL DEFAULT 0,
    density_g_cm3 DOUBLE PRECISION NOT NULL DEFAULT 1.24,
    purchased_at TIMESTAMPTZ,
    archived INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE printers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('moonraker')),
    moonraker_url TEXT NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    active_spool_id TEXT REFERENCES spools(id),
    state TEXT NOT NULL DEFAULT 'unknown',
    current_file TEXT NOT NULL DEFAULT '',
    progress DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE print_jobs (
    id TEXT PRIMARY KEY,
    spool_id TEXT REFERENCES spools(id),
    printer_id TEXT REFERENCES printers(id),
    external_print_id TEXT NOT NULL DEFAULT '',
    gcode_filename TEXT NOT NULL,
    model_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('success', 'failed', 'cancelled')),
    completion_percent INTEGER NOT NULL DEFAULT 100,
    estimated_weight_g DOUBLE PRECISION NOT NULL,
    actual_weight_g DOUBLE PRECISION NOT NULL,
    estimated_length_mm DOUBLE PRECISION NOT NULL DEFAULT 0,
    print_duration_sec BIGINT NOT NULL DEFAULT 0,
    nozzle_temp_c INTEGER NOT NULL DEFAULT 0,
    bed_temp_c INTEGER NOT NULL DEFAULT 0,
    slicer TEXT NOT NULL DEFAULT '',
    preview_image_path TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    printed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    usage_measured INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_print_jobs_printer_external_print_id
    ON print_jobs(printer_id, external_print_id)
    WHERE printer_id IS NOT NULL AND external_print_id != '';
CREATE INDEX idx_print_jobs_spool_id ON print_jobs(spool_id);
CREATE INDEX idx_print_jobs_printer_id ON print_jobs(printer_id);
CREATE INDEX idx_print_jobs_status ON print_jobs(status);
CREATE INDEX idx_print_jobs_printed_at ON print_jobs(printed_at);

CREATE TABLE drying_sessions (
    id TEXT PRIMARY KEY,
    spool_id TEXT NOT NULL REFERENCES spools(id),
    temperature_c INTEGER NOT NULL,
    duration_hours DOUBLE PRECISION NOT NULL,
    dried_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_drying_sessions_spool_id ON drying_sessions(spool_id);
CREATE INDEX idx_drying_sessions_dried_at ON drying_sessions(dried_at);

CREATE TABLE printer_models (
    printer_id TEXT NOT NULL REFERENCES printers(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    modified_at TIMESTAMPTZ,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    estimated_weight_g DOUBLE PRECISION NOT NULL DEFAULT 0,
    print_duration_sec BIGINT NOT NULL DEFAULT 0,
    slicer TEXT NOT NULL DEFAULT '',
    preview_image_path TEXT NOT NULL DEFAULT '',
    cached_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (printer_id, path)
);
CREATE INDEX idx_printer_models_cached_at ON printer_models(cached_at);

-- +goose Down
DROP TABLE printer_models;
DROP TABLE drying_sessions;
DROP TABLE print_jobs;
DROP TABLE printers;
DROP TABLE spools;
DROP TABLE app_settings;
DROP TABLE materials;
