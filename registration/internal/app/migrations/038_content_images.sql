-- Images uploaded from the mobile content editor (speaker portraits). Each is
-- stored once under its content hash, so its public URL never changes meaning
-- and can be cached forever by browsers, the app and CloudFront.
CREATE TABLE content_images (
 id text PRIMARY KEY,
 kind text NOT NULL CHECK(kind IN ('portrait')),
 mime text NOT NULL CHECK(mime IN ('image/webp','image/jpeg')),
 size integer NOT NULL CHECK(size > 0),
 created_by text NOT NULL REFERENCES staff,
 created_at timestamptz NOT NULL DEFAULT now()
);
