-- Staff assign each exhibitor its stall on the expo floor. Empty means not yet
-- allocated; the public directory and mobile app show "Stall to be announced".
ALTER TABLE registrations ADD COLUMN stall_number text NOT NULL DEFAULT ''
 CONSTRAINT registration_stall_number CHECK(stall_number = btrim(stall_number) AND length(stall_number) <= 16);
