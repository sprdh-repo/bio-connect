-- Who owns a template. Twenty-four ship in the binary and `poster-seed
-- --replace` rolls out new artwork; without this it would also revert any
-- layout staff had changed in the builder, silently and on every deploy.
--
-- A template starts as 'seed' only when the seeder inserts it. Saving any edit
-- in the console moves it to 'staff', and the seeder then leaves it alone.
ALTER TABLE poster_templates
 ADD COLUMN origin text NOT NULL DEFAULT 'staff' CHECK(origin IN ('seed','staff'));

-- Templates already installed by an earlier seeder run are seed-owned. They are
-- recognisable because their artwork layers point at assets the seeder labelled
-- with its own prefix; anything staff built points at an ordinary upload.
UPDATE poster_templates t SET origin='seed'
 WHERE EXISTS (
   SELECT 1 FROM jsonb_array_elements(t.spec->'layers') l
   JOIN poster_assets a ON a.id = l->>'asset_id'
   WHERE a.label LIKE 'seed:%'
 );
