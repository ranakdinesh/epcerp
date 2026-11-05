ALTER TABLE user_profile ADD COLUMN photo_object_key text, ADD COLUMN photo_sha256 text;
ALTER TABLE user_profile ADD CONSTRAINT user_profile_image_reference CHECK (
 (photo_object_key IS NULL AND photo_sha256 IS NULL) OR
 (photo_object_key IS NOT NULL AND length(photo_object_key) BETWEEN 1 AND 512
  AND photo_sha256 IS NOT NULL AND photo_sha256 ~ '^[0-9a-f]{64}$' AND photo IS NULL)
);
