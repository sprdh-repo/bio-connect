-- Self-service kiosk. A kiosk runs on an ops session flagged here, which can only
-- reach the kiosk endpoints: an unattended tablet never holds a session that can
-- read the roster, undo attendance or run gates. Staff open and close a kiosk with
-- the shared passcode, and both are kept in the activity audit.
ALTER TABLE ops_sessions ADD COLUMN kiosk boolean NOT NULL DEFAULT false;

ALTER TABLE ops_activity DROP CONSTRAINT ops_activity_kind_check;
ALTER TABLE ops_activity ADD CONSTRAINT ops_activity_kind_check CHECK(kind IN (
 'check_in','repeat_check_in','check_in_denied','undo_check_in',
 'check_out','repeat_check_out','check_out_denied','undo_check_out',
 'badge_print','gate_allow','gate_deny','gate_undo','spot_registration',
 'kiosk_open','kiosk_close'
));
