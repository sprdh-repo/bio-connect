-- open governs paid registration; free_open separately governs free links, so
-- a category can be closed to complimentary registrations without closing it
-- to paying registrants (or the reverse).
ALTER TABLE categories
 ADD COLUMN free_open boolean NOT NULL DEFAULT true;

UPDATE categories SET free_open=false WHERE id='student';
