-- +goose Up
CREATE TABLE spools (
    id            TEXT PRIMARY KEY,
    manufacturer  TEXT NOT NULL,
    material      TEXT NOT NULL,
    color         TEXT NOT NULL,
    color_hex     TEXT NOT NULL DEFAULT '#000000',
    diameter_mm   REAL NOT NULL DEFAULT 1.75,
    total_weight_g REAL NOT NULL,
    price         REAL NOT NULL DEFAULT 0,
    density_g_cm3 REAL NOT NULL DEFAULT 1.24,
    purchased_at  DATETIME,
    archived      INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE print_jobs (
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

CREATE TABLE drying_sessions (
    id             TEXT PRIMARY KEY,
    spool_id       TEXT NOT NULL REFERENCES spools(id),
    temperature_c  INTEGER NOT NULL,
    duration_hours REAL NOT NULL,
    notes          TEXT NOT NULL DEFAULT '',
    dried_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_print_jobs_spool_id ON print_jobs(spool_id);
CREATE INDEX idx_print_jobs_status ON print_jobs(status);
CREATE INDEX idx_print_jobs_printed_at ON print_jobs(printed_at);
CREATE INDEX idx_drying_sessions_spool_id ON drying_sessions(spool_id);
CREATE INDEX idx_drying_sessions_dried_at ON drying_sessions(dried_at);

-- +goose Down
DROP TABLE IF EXISTS drying_sessions;
DROP TABLE IF EXISTS print_jobs;
DROP TABLE IF EXISTS spools;
