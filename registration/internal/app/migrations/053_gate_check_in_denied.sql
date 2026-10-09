-- A gate can also check in known attendees it denies, so a counter that turns
-- people away (for example a kit desk for one category) still records them as
-- on site for the day.
ALTER TABLE access_points ADD COLUMN check_in_denied boolean NOT NULL DEFAULT false;
