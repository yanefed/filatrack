-- +goose Up
ALTER TABLE custom_materials
    ADD COLUMN drying_profile_material TEXT
    REFERENCES drying_profiles(material) ON UPDATE CASCADE ON DELETE SET NULL;

UPDATE custom_materials
SET drying_profile_material = name
WHERE EXISTS (
    SELECT 1 FROM drying_profiles WHERE drying_profiles.material = custom_materials.name
);

CREATE INDEX idx_custom_materials_drying_profile
    ON custom_materials(drying_profile_material);

-- +goose Down
DROP INDEX IF EXISTS idx_custom_materials_drying_profile;
ALTER TABLE custom_materials DROP COLUMN drying_profile_material;
