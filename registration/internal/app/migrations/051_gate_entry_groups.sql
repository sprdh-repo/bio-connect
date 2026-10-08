-- One-entry gates sharing an entry group allow one entry per attendee per day
-- across the whole group, e.g. several food counters serving one lunch.
ALTER TABLE access_points ADD COLUMN entry_group text NOT NULL DEFAULT ''
 CHECK(length(entry_group) <= 60);
ALTER TABLE access_points ADD CONSTRAINT access_points_entry_group_one_entry
 CHECK(entry_group = '' OR NOT allow_multiple_entries);
CREATE INDEX access_points_entry_group ON access_points(entry_group) WHERE entry_group <> '';
