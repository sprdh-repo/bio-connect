-- A staff override admits an attendee a gate denied. It is recorded as its own
-- allowed scan pointing at the denied one; each denial is overridden at most once.
ALTER TABLE access_scans ADD COLUMN override_of text REFERENCES access_scans(id);
CREATE UNIQUE INDEX access_scans_override ON access_scans(override_of) WHERE override_of IS NOT NULL;

ALTER TABLE ops_activity DROP CONSTRAINT ops_activity_kind_check;
ALTER TABLE ops_activity ADD CONSTRAINT ops_activity_kind_check CHECK(kind IN (
 'check_in','repeat_check_in','check_in_denied','undo_check_in',
 'check_out','repeat_check_out','check_out_denied','undo_check_out',
 'badge_print','gate_allow','gate_deny','gate_undo','spot_registration',
 'kiosk_open','kiosk_close','badge_lookup','badge_lookup_denied','kiosk_scan',
 'gate_created','gate_opened','gate_closed','gate_rules_updated','gate_override'
));
