-- +goose Up
CREATE TABLE print_jobs_new (
    id                  TEXT PRIMARY KEY,
    spool_id            TEXT REFERENCES spools(id),
    printer_id          TEXT REFERENCES printers(id),
    external_print_id   TEXT NOT NULL DEFAULT '',
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
    preview_image_path  TEXT NOT NULL DEFAULT '',
    notes               TEXT NOT NULL DEFAULT '',
    printed_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO print_jobs_new (
    id, spool_id, printer_id, external_print_id, gcode_filename, model_name,
    status, completion_percent, estimated_weight_g, actual_weight_g,
    estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c,
    slicer, preview_image_path, notes, printed_at, created_at
)
SELECT
    id, spool_id, printer_id, external_print_id, gcode_filename, model_name,
    status, completion_percent, estimated_weight_g, actual_weight_g,
    estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c,
    slicer, preview_image_path, notes, printed_at, created_at
FROM print_jobs;

DROP TABLE print_jobs;
ALTER TABLE print_jobs_new RENAME TO print_jobs;

CREATE INDEX idx_print_jobs_spool_id ON print_jobs(spool_id);
CREATE INDEX idx_print_jobs_status ON print_jobs(status);
CREATE INDEX idx_print_jobs_printed_at ON print_jobs(printed_at);
CREATE INDEX idx_print_jobs_printer_id ON print_jobs(printer_id);
CREATE UNIQUE INDEX idx_print_jobs_printer_external_print_id
    ON print_jobs(printer_id, external_print_id)
    WHERE printer_id IS NOT NULL AND external_print_id != '';

-- +goose Down
CREATE TABLE print_jobs_old (
    id                  TEXT PRIMARY KEY,
    spool_id            TEXT NOT NULL REFERENCES spools(id),
    printer_id          TEXT REFERENCES printers(id),
    external_print_id   TEXT NOT NULL DEFAULT '',
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
    preview_image_path  TEXT NOT NULL DEFAULT '',
    notes               TEXT NOT NULL DEFAULT '',
    printed_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO print_jobs_old (
    id, spool_id, printer_id, external_print_id, gcode_filename, model_name,
    status, completion_percent, estimated_weight_g, actual_weight_g,
    estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c,
    slicer, preview_image_path, notes, printed_at, created_at
)
SELECT
    id, spool_id, printer_id, external_print_id, gcode_filename, model_name,
    status, completion_percent, estimated_weight_g, actual_weight_g,
    estimated_length_mm, print_duration_sec, nozzle_temp_c, bed_temp_c,
    slicer, preview_image_path, notes, printed_at, created_at
FROM print_jobs
WHERE spool_id IS NOT NULL;

DROP TABLE print_jobs;
ALTER TABLE print_jobs_old RENAME TO print_jobs;

CREATE INDEX idx_print_jobs_spool_id ON print_jobs(spool_id);
CREATE INDEX idx_print_jobs_status ON print_jobs(status);
CREATE INDEX idx_print_jobs_printed_at ON print_jobs(printed_at);
CREATE INDEX idx_print_jobs_printer_id ON print_jobs(printer_id);
CREATE UNIQUE INDEX idx_print_jobs_printer_external_print_id
    ON print_jobs(printer_id, external_print_id)
    WHERE printer_id IS NOT NULL AND external_print_id != '';
