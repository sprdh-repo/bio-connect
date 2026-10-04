-- Retain rejected badge-desk scans alongside successful and repeated activity.
-- Migration 022 includes these values for fresh databases; this upgrades databases
-- where the on-site operations schema has already been applied.
ALTER TABLE ops_activity DROP CONSTRAINT ops_activity_kind_check;
ALTER TABLE ops_activity ADD CONSTRAINT ops_activity_kind_check CHECK(kind IN (
 'check_in','repeat_check_in','check_in_denied','undo_check_in',
 'check_out','repeat_check_out','check_out_denied','undo_check_out',
 'badge_print','gate_allow','gate_deny','gate_undo','spot_registration'
));
