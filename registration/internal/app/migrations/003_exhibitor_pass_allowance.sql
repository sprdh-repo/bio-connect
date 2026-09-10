-- Exhibitor pass allowances change from 6 / 4 / 2 to 3 / 2 / 2 (premium,
-- standard, table), and the category labels drop the provisional stall sizes.
-- Registrations already made keep the roster they submitted: each one now
-- records its own roster size, and approval checks against that rather than
-- the category's current allowance.
ALTER TABLE registrations ADD COLUMN roster_count integer;
UPDATE registrations r SET roster_count=(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id);
ALTER TABLE registrations ALTER COLUMN roster_count SET NOT NULL,
 ADD CONSTRAINT registration_roster_count CHECK(roster_count BETWEEN 1 AND 6);
UPDATE categories SET roster_count=3, label='Premium stall' WHERE id='premium';
UPDATE categories SET roster_count=2, label='Standard stall' WHERE id='standard';
UPDATE categories SET label='Table space' WHERE id='table';
