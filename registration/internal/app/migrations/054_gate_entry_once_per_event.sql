-- A one-entry gate normally resets each event day. With entry_once_per_event
-- the limit spans the whole event, e.g. a kit counter that hands out one kit
-- per attendee in total.
ALTER TABLE access_points ADD COLUMN entry_once_per_event boolean NOT NULL DEFAULT false;
ALTER TABLE access_points ADD CONSTRAINT access_points_once_per_event_one_entry
 CHECK(NOT entry_once_per_event OR NOT allow_multiple_entries);
