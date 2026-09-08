CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE categories (
 id text PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('delegate','exhibitor')), label text NOT NULL,
 early_paise bigint NOT NULL CHECK(early_paise > 0), regular_paise bigint NOT NULL CHECK(regular_paise >= early_paise),
 roster_count integer NOT NULL CHECK(roster_count BETWEEN 1 AND 6), open boolean NOT NULL DEFAULT true
);
INSERT INTO categories VALUES
 ('student','delegate','Students',100000,150000,1,true),
 ('startup','delegate','Incubation / Startups',350000,400000,1,true),
 ('faculty','delegate','Faculty / Scientists',400000,450000,1,true),
 ('industry','delegate','Industry',600000,700000,1,true),
 ('premium','exhibitor','Premium stall (6m × 3m)',20000000,20000000,6,true),
 ('standard','exhibitor','Standard stall (3m × 2m)',5000000,5000000,4,true),
 ('table','exhibitor','Table space (2m × 2m)',2000000,2000000,2,true);
CREATE TABLE registrations (
 id text PRIMARY KEY, reference text NOT NULL UNIQUE, idempotency_hash text NOT NULL UNIQUE, request_hash text NOT NULL,
 category_id text NOT NULL REFERENCES categories, institution text NOT NULL, contact_name text NOT NULL,
 email text NOT NULL, phone text NOT NULL, description text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'awaiting_payment' CHECK(status IN ('awaiting_payment','awaiting_review','correction_requested','approved','rejected','cancelled')),
 quoted_paise bigint NOT NULL CHECK(quoted_paise>0), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 approved_at timestamptz, approved_by text, review_note text NOT NULL DEFAULT '', management_hash text NOT NULL,
 management_expires timestamptz NOT NULL, contact_pack_hash text, contact_pack_cipher text
);
CREATE INDEX registration_search ON registrations(created_at DESC);
CREATE INDEX registration_email ON registrations(lower(email));
CREATE TABLE attendees (
 id text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations, name text NOT NULL, email text NOT NULL,
 phone text NOT NULL, designation text NOT NULL, whatsapp_consent boolean NOT NULL DEFAULT false, consent_at timestamptz,
 consent_text text NOT NULL DEFAULT '', position integer NOT NULL, UNIQUE(registration_id,position)
);
CREATE TABLE files (
 id text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations, kind text NOT NULL CHECK(kind IN ('logo','receipt','pass')),
 object_key text NOT NULL UNIQUE, mime text NOT NULL, size bigint NOT NULL CHECK(size>0), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE payment_submissions (
 id text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations, bank_reference text NOT NULL,
 payment_date date NOT NULL, amount_paise bigint NOT NULL CHECK(amount_paise>0), receipt_id text NOT NULL REFERENCES files,
 created_at timestamptz NOT NULL DEFAULT now(), verified_at timestamptz, verified_by text,
 verified_date date, verified_amount_paise bigint, verified_reference text, beneficiary_confirmed boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX approved_transaction ON payment_submissions(verified_reference) WHERE verified_at IS NOT NULL;
CREATE TABLE staff (
 id text PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL, totp_cipher text NOT NULL,
 role text NOT NULL CHECK(role IN ('manager','reviewer')), active boolean NOT NULL DEFAULT true,
 last_totp_step bigint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE registrations ADD FOREIGN KEY(approved_by) REFERENCES staff;
ALTER TABLE payment_submissions ADD FOREIGN KEY(verified_by) REFERENCES staff;
CREATE TABLE sessions (
 token_hash text PRIMARY KEY, staff_id text NOT NULL REFERENCES staff, csrf_hash text NOT NULL,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE rate_limits (key text PRIMARY KEY, attempts integer NOT NULL DEFAULT 1, expires_at timestamptz NOT NULL);
CREATE TABLE recovery_tokens (
 token_hash text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations,
 expires_at timestamptz NOT NULL, used_at timestamptz
);
CREATE TABLE passes (
 id text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations, attendee_id text NOT NULL REFERENCES attendees,
 version integer NOT NULL CHECK(version>0), number text NOT NULL UNIQUE, qr_id text NOT NULL UNIQUE,
 download_hash text NOT NULL UNIQUE, download_cipher text NOT NULL, file_id text REFERENCES files,
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz, UNIQUE(attendee_id,version)
);
CREATE UNIQUE INDEX one_active_pass ON passes(attendee_id) WHERE revoked_at IS NULL;
CREATE TABLE delivery_jobs (
 id text PRIMARY KEY, registration_id text NOT NULL REFERENCES registrations, pass_id text REFERENCES passes,
 purpose text NOT NULL CHECK(purpose IN ('pass','pack','recovery','registration')),
 channel text NOT NULL CHECK(channel IN ('email','whatsapp')), recipient text NOT NULL,
 payload_cipher text NOT NULL DEFAULT '', dedupe_key text NOT NULL UNIQUE,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','sending','accepted','delivered','failed','uncertain','cancelled')),
 attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), claimed_at timestamptz,
 provider_id text, error_code text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX queue_ready ON delivery_jobs(available_at) WHERE status='queued';
CREATE UNIQUE INDEX provider_message ON delivery_jobs(channel,provider_id) WHERE provider_id IS NOT NULL;
CREATE TABLE webhook_events (
 event_hash text PRIMARY KEY, channel text NOT NULL, provider_id text NOT NULL, status text NOT NULL, received_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE audit_events (
 id bigserial PRIMARY KEY, staff_id text REFERENCES staff, registration_id text REFERENCES registrations,
 action text NOT NULL, detail text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations(version) VALUES(1);
