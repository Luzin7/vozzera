-- +goose Up
ALTER TABLE rooms ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE rooms ADD COLUMN has_voice BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE rooms ADD COLUMN staff_only BOOLEAN NOT NULL DEFAULT false;

UPDATE rooms SET has_voice = true WHERE type = 'voice';

ALTER TABLE rooms DROP COLUMN type;

-- +goose Down
ALTER TABLE rooms ADD COLUMN type VARCHAR(20);

UPDATE rooms SET type = CASE WHEN has_voice THEN 'voice' ELSE 'text' END;

ALTER TABLE rooms ALTER COLUMN type SET NOT NULL;

ALTER TABLE rooms DROP COLUMN staff_only;
ALTER TABLE rooms DROP COLUMN has_voice;
ALTER TABLE rooms DROP COLUMN created_by;
