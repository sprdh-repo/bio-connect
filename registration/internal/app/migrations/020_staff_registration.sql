-- Staff can register delegates and exhibitors directly from the console
-- (StaffCreate). Such a registration is approved at once, either against a
-- bank payment staff verified or as complimentary. created_by records the
-- staff member; complimentary marks a free registration that did not come
-- through a free link.
ALTER TABLE registrations
 ADD COLUMN created_by text REFERENCES staff,
 ADD COLUMN complimentary boolean NOT NULL DEFAULT false;
