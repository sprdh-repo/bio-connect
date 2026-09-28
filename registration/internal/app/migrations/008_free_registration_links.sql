-- Staff can issue a revocable invitation link that makes any delegate category
-- free until the end of a chosen date. Only hashes are retained, so an old link
-- cannot be recovered from the database after a replacement is generated.
CREATE TABLE free_registration_links (
 id text PRIMARY KEY,
 token_hash text NOT NULL UNIQUE,
 expires_at timestamptz NOT NULL,
 auto_approve boolean NOT NULL DEFAULT true,
 created_by text NOT NULL REFERENCES staff,
 created_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz,
 revoked_by text REFERENCES staff,
 CHECK(revoked_at IS NOT NULL OR revoked_by IS NULL)
);

ALTER TABLE registrations
 DROP CONSTRAINT registrations_quoted_paise_check,
 ADD CONSTRAINT registrations_quoted_paise_check CHECK(quoted_paise >= 0),
 ADD COLUMN free_link_id text REFERENCES free_registration_links;

CREATE INDEX free_registration_links_created_at ON free_registration_links(created_at DESC);
CREATE INDEX registrations_free_link_id ON registrations(free_link_id) WHERE free_link_id IS NOT NULL;
