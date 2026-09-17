-- Social posters. Comms produce speaker reveals, session announces, countdowns
-- and partner welcomes by hand today; this lets staff build a template once and
-- then fill it per post inside the console.
--
-- A template is an ordered layer stack rather than a background plus an overlay,
-- because the artwork puts decoration above the portrait and the caption above
-- that. Order in spec->'layers' is draw order, so staff reorder rather than the
-- renderer guessing.
--
-- Poster assets are their own table: files.registration_id is NOT NULL and a
-- poster belongs to no registration.
CREATE TABLE poster_assets (
 id text PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('art','photo','logo')),
 label text NOT NULL DEFAULT '', object_key text NOT NULL UNIQUE, mime text NOT NULL,
 width integer NOT NULL CHECK(width>0), height integer NOT NULL CHECK(height>0),
 size bigint NOT NULL CHECK(size>0),
 created_by text REFERENCES staff, created_at timestamptz NOT NULL DEFAULT now()
);

-- One row per output size. A family ("speaker-reveal") groups the sizes that
-- share field names, so a poster is filled once and exported three ways.
CREATE TABLE poster_templates (
 id text PRIMARY KEY, family text NOT NULL CHECK(family<>''), name text NOT NULL,
 size text NOT NULL CHECK(size IN ('4x5','1x1','9x16')),
 width integer NOT NULL CHECK(width BETWEEN 1 AND 4096),
 height integer NOT NULL CHECK(height BETWEEN 1 AND 4096),
 spec jsonb NOT NULL, active boolean NOT NULL DEFAULT true,
 created_by text REFERENCES staff,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX poster_template_size ON poster_templates(family,size) WHERE active;

-- content holds the filled fields plus the per-size photo framing: the same
-- portrait needs a different crop at 4:5 and at 9:16.
CREATE TABLE posters (
 id text PRIMARY KEY, family text NOT NULL, title text NOT NULL DEFAULT '',
 content jsonb NOT NULL,
 created_by text REFERENCES staff,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX posters_recent ON posters(created_at DESC,id);
