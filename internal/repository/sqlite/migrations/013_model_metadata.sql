-- +goose Up
ALTER TABLE printer_models ADD COLUMN metadata_ready BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE printer_models ADD COLUMN gcode_parsed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE printer_models ADD COLUMN source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE printer_models ADD COLUMN filament_weight_g DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE printer_models ADD COLUMN filament_length_mm DOUBLE PRECISION NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE printer_models DROP COLUMN filament_length_mm;
ALTER TABLE printer_models DROP COLUMN filament_weight_g;
ALTER TABLE printer_models DROP COLUMN source_url;
ALTER TABLE printer_models DROP COLUMN gcode_parsed;
ALTER TABLE printer_models DROP COLUMN metadata_ready;
