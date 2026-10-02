-- Staff can remove an attendee from a registration. Passes are never deleted
-- (pass numbers count every pass ever issued), and a pass references its
-- holder, so a removed attendee is kept and marked rather than deleted; their
-- pass is revoked at the same time.
ALTER TABLE attendees ADD COLUMN removed_at timestamptz;
