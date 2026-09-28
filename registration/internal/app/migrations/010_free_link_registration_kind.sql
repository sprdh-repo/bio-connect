ALTER TABLE free_registration_links
 ADD COLUMN registration_kind text NOT NULL DEFAULT 'delegate'
 CHECK(registration_kind IN ('delegate','exhibitor'));

-- Migration 008 allowed an expired, unrevoked link to remain when its
-- replacement was generated. Keep only the newest such link before enforcing
-- one independently managed current link per registration type.
WITH superseded AS (
 SELECT id FROM (
  SELECT id,row_number() OVER (PARTITION BY registration_kind ORDER BY created_at DESC,id DESC) AS position
  FROM free_registration_links WHERE revoked_at IS NULL
 ) links WHERE position > 1
)
UPDATE free_registration_links l
 SET revoked_at=now(),revoked_by=l.created_by
 FROM superseded s WHERE l.id=s.id;

CREATE UNIQUE INDEX free_registration_links_active_kind
 ON free_registration_links(registration_kind)
 WHERE revoked_at IS NULL;
