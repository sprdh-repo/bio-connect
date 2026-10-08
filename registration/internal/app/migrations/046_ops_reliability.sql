ALTER TABLE ops_activity DROP CONSTRAINT ops_activity_kind_check;
ALTER TABLE ops_activity ADD CONSTRAINT ops_activity_kind_check CHECK(kind IN (
 'check_in','repeat_check_in','check_in_denied','undo_check_in',
 'check_out','repeat_check_out','check_out_denied','undo_check_out',
 'badge_print','gate_allow','gate_deny','gate_undo','spot_registration',
 'kiosk_open','kiosk_close','badge_lookup','badge_lookup_denied','kiosk_scan',
 'gate_created','gate_opened','gate_closed'
));

-- Replays return the committed decision, never another entry/exit transition.
ALTER TABLE access_scans ADD COLUMN request_id text;
ALTER TABLE access_scans ADD COLUMN request_hash text;
ALTER TABLE access_scans ADD COLUMN response jsonb;
CREATE UNIQUE INDEX access_scan_request ON access_scans(access_point_id,station,request_id) WHERE request_id IS NOT NULL;

-- Retrying a lost authorization response does not consume another badge/reprint.
-- Authorization belongs to this session, not another device with the same name.
CREATE TABLE kiosk_print_requests (
 session_hash text NOT NULL REFERENCES ops_sessions(token_hash) ON DELETE CASCADE,
 request_id text NOT NULL,
 attendee_id text NOT NULL REFERENCES attendees,
 event_day date NOT NULL,
 retry boolean NOT NULL,
 first_check_in boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(session_hash,request_id)
);
