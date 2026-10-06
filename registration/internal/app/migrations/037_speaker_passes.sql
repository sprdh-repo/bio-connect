-- Speakers get a complimentary one-person pass that staff issue from the
-- Speakers console. It is staff-only: closed to paid registration and to free
-- links, so no public form offers it.
INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open)
 VALUES('speaker','delegate','Speaker',1,1,1,false,true,false);

-- Links a speaker in the directory to the registration that holds their pass.
-- The mobile content editor rewrites the speakers table on every save, so this
-- is keyed by the speaker's stable id rather than a foreign key to it. The
-- pass holder's email and phone live on the attendee, as for every other pass.
CREATE TABLE speaker_registrations (
 speaker_id text PRIMARY KEY,
 registration_id text NOT NULL UNIQUE REFERENCES registrations,
 created_at timestamptz NOT NULL DEFAULT now()
);
