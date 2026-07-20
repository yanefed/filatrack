-- +goose Up
ALTER TABLE spools ADD COLUMN display_code TEXT NOT NULL DEFAULT '';

UPDATE spools
SET display_code =
    upper(substr(material, 1, 3)) || '-' ||
    upper(substr(replace(color, ' ', ''), 1, 3)) || '-' ||
    upper(substr(replace(manufacturer, ' ', ''), 1, 3)) || '-' ||
    upper(substr(replace(id, '-', ''), -4, 4))
WHERE display_code = '';

CREATE UNIQUE INDEX idx_spools_display_code ON spools(display_code);

CREATE TABLE printers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN ('moonraker')),
    moonraker_url   TEXT NOT NULL,
    api_key         TEXT NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    active_spool_id TEXT REFERENCES spools(id),
    state           TEXT NOT NULL DEFAULT 'unknown',
    current_file    TEXT NOT NULL DEFAULT '',
    progress        REAL NOT NULL DEFAULT 0,
    last_seen_at    DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_printers_active_spool_id ON printers(active_spool_id);

ALTER TABLE print_jobs ADD COLUMN printer_id TEXT REFERENCES printers(id);
ALTER TABLE print_jobs ADD COLUMN external_print_id TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_print_jobs_printer_id ON print_jobs(printer_id);
CREATE UNIQUE INDEX idx_print_jobs_printer_external_print_id
    ON print_jobs(printer_id, external_print_id)
    WHERE printer_id IS NOT NULL AND external_print_id != '';

-- +goose Down
DROP INDEX IF EXISTS idx_print_jobs_printer_external_print_id;
DROP INDEX IF EXISTS idx_print_jobs_printer_id;
DROP INDEX IF EXISTS idx_printers_active_spool_id;
DROP TABLE IF EXISTS printers;
DROP INDEX IF EXISTS idx_spools_display_code;

CREATE TABLE print_jobs_old (
    id                  TEXT PRIMARY KEY,
    spool_id            TEXT NOT NULL REFERENCES spools(id),
    gcode_filename      TEXT NOT NULL,
    model_name          TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL CHECK (status IN ('success', 'failed', 'cancelled')),
    completion_percent  INTEGER NOT NULL DEFAULT 100,
    estimated_weight_g  REAL NOT NULL,
    actual_weight_g     REAL NOT NULL,
    estimated_length_mm REAL NOT NULL DEFAULT 0,
    print_duration_sec  INTEGER NOT NULL DEFAULT 0,
    nozzle_temp_c       INTEGER NOT NULL DEFAULT 0,
    bed_temp_c          INTEGER NOT NULL DEFAULT 0,
    slicer              TEXT NOT NULL DEFAULT '',
    notes               TEXT NOT NULL DEFAULT '',
    printed_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO print_jobs_old (
    id, spool_id, gcode_filename, model_name, status, completion_percent,
    estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec,
    nozzle_temp_c, bed_temp_c, slicer, notes, printed_at, created_at
)
SELECT
    id, spool_id, gcode_filename, model_name, status, completion_percent,
    estimated_weight_g, actual_weight_g, estimated_length_mm, print_duration_sec,
    nozzle_temp_c, bed_temp_c, slicer, notes, printed_at, created_at
FROM print_jobs;
DROP TABLE print_jobs;
ALTER TABLE print_jobs_old RENAME TO print_jobs;
CREATE INDEX idx_print_jobs_spool_id ON print_jobs(spool_id);
CREATE INDEX idx_print_jobs_status ON print_jobs(status);
CREATE INDEX idx_print_jobs_printed_at ON print_jobs(printed_at);

CREATE TABLE spools_old (
    id             TEXT PRIMARY KEY,
    manufacturer   TEXT NOT NULL,
    material       TEXT NOT NULL,
    color          TEXT NOT NULL,
    color_hex      TEXT NOT NULL DEFAULT '#000000',
    diameter_mm    REAL NOT NULL DEFAULT 1.75,
    total_weight_g REAL NOT NULL,
    price          REAL NOT NULL DEFAULT 0,
    density_g_cm3  REAL NOT NULL DEFAULT 1.24,
    purchased_at   DATETIME,
    archived       INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO spools_old (
    id, manufacturer, material, color, color_hex, diameter_mm, total_weight_g,
    price, density_g_cm3, purchased_at, archived, created_at, updated_at
)
SELECT
    id, manufacturer, material, color, color_hex, diameter_mm, total_weight_g,
    price, density_g_cm3, purchased_at, archived, created_at, updated_at
FROM spools;
DROP TABLE spools;
ALTER TABLE spools_old RENAME TO spools;
