-- On-site operations are deliberately separate from registration state. A desk can
-- admit, undo, check out, print and operate access points without mutating an approved
-- registration or its pass-delivery history.
INSERT INTO staff(id,email,password_hash,totp_cipher,role,active)
VALUES('ops-system','ops-system@bioconnect.invalid','disabled','disabled','reviewer',false)
ON CONFLICT(id) DO NOTHING;

CREATE TABLE ops_sessions (
 token_hash text PRIMARY KEY,
 csrf_hash text NOT NULL,
 station text NOT NULL CHECK(length(station) BETWEEN 1 AND 60),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ops_attendance (
 attendee_id text NOT NULL REFERENCES attendees,
 event_day date NOT NULL CHECK(event_day IN ('2026-10-08','2026-10-09')),
 checked_in_at timestamptz NOT NULL DEFAULT now(),
 checked_in_by text NOT NULL,
 checked_out_at timestamptz,
 checked_out_by text,
 PRIMARY KEY(attendee_id,event_day),
 CHECK((checked_out_at IS NULL)=(checked_out_by IS NULL))
);
CREATE INDEX ops_attendance_day ON ops_attendance(event_day,checked_in_at);

CREATE TABLE ops_activity (
 id text PRIMARY KEY,
 attendee_id text REFERENCES attendees,
 event_day date,
 kind text NOT NULL CHECK(kind IN ('check_in','repeat_check_in','check_in_denied','undo_check_in','check_out','repeat_check_out','check_out_denied','undo_check_out','badge_print','gate_allow','gate_deny','gate_undo','spot_registration')),
 station text NOT NULL,
 detail text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ops_activity_day ON ops_activity(event_day,created_at DESC);

CREATE TABLE access_points (
 id text PRIMARY KEY,
 name text NOT NULL UNIQUE,
 mode text NOT NULL CHECK(mode IN ('enforce','log')),
 direction text NOT NULL CHECK(direction IN ('entry','exit','auto')),
 allowed_categories text[] NOT NULL DEFAULT '{}',
 allowed_days date[] NOT NULL DEFAULT ARRAY['2026-10-08'::date,'2026-10-09'::date],
 capacity integer CHECK(capacity>0),
 require_check_in boolean NOT NULL DEFAULT true,
 allow_multiple_entries boolean NOT NULL DEFAULT true,
 active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE access_scans (
 id text PRIMARY KEY,
 access_point_id text NOT NULL REFERENCES access_points,
 attendee_id text REFERENCES attendees,
 event_day date NOT NULL,
 reference text NOT NULL,
 direction text NOT NULL CHECK(direction IN ('entry','exit')),
 decision text NOT NULL CHECK(decision IN ('allow','deny')),
 reason text NOT NULL DEFAULT '',
 would_deny text NOT NULL DEFAULT '',
 station text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 voided_at timestamptz,
 voided_by text,
 void_reason text NOT NULL DEFAULT ''
);
CREATE INDEX access_scans_point ON access_scans(access_point_id,created_at);
CREATE INDEX access_scans_attendee ON access_scans(access_point_id,attendee_id,created_at) WHERE voided_at IS NULL;
