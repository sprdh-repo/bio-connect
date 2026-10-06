-- A speaker's contact details, kept before (and independently of) their pass.
-- Staff collect emails first and issue passes later, so saving an email no
-- longer issues a pass. Once a pass exists its holder (the attendee) is kept
-- in step with this row. Keyed by speaker id for the same reason as
-- speaker_registrations: the content editor rewrites the speakers table.
CREATE TABLE speaker_contacts (
 speaker_id text PRIMARY KEY,
 email text NOT NULL,
 phone text NOT NULL DEFAULT '',
 whatsapp_consent boolean NOT NULL DEFAULT false,
 updated_by text REFERENCES staff,
 updated_at timestamptz NOT NULL DEFAULT now()
);
