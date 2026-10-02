-- Staff can give an exhibitor more passes than its stall type includes
-- (SetAllowance), so a registration's allowance is no longer capped at the
-- largest stall's. Category allowances keep their own limit.
ALTER TABLE registrations
 DROP CONSTRAINT registration_roster_count,
 ADD CONSTRAINT registration_roster_count CHECK(roster_count BETWEEN 1 AND 50);
