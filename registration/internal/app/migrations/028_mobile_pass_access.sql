-- Attendee access is separate from staff and registration-management sessions.
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_purpose_check,
 ADD CONSTRAINT delivery_jobs_purpose_check CHECK(purpose IN ('pass','pack','recovery','registration','payment_reminder','roster_notice','pass_otp'));
CREATE TABLE mobile_pass_challenges (
 token_hash text PRIMARY KEY,
 channel text NOT NULL CHECK(channel IN ('email','whatsapp')),
 identifier text NOT NULL,
 qr_id text NOT NULL DEFAULT '',
 code_hash text NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 expires_at timestamptz NOT NULL,
 used_at timestamptz,
 delivery_id text UNIQUE REFERENCES delivery_jobs,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mobile_challenge_expiry ON mobile_pass_challenges(expires_at);
CREATE TABLE mobile_pass_sessions (
 token_hash text PRIMARY KEY,
 channel text NOT NULL CHECK(channel IN ('email','whatsapp')),
 identifier text NOT NULL,
 qr_id text NOT NULL DEFAULT '',
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mobile_session_expiry ON mobile_pass_sessions(expires_at);
CREATE INDEX attendee_email_lookup ON attendees(lower(email)) WHERE removed_at IS NULL;
CREATE INDEX attendee_phone_lookup ON attendees(phone) WHERE removed_at IS NULL;
