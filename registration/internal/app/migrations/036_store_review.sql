-- Reviewer preferences live only in the reviewer's authenticated session.
ALTER TABLE mobile_pass_sessions
 ADD COLUMN review_share_email boolean NOT NULL DEFAULT false,
 ADD COLUMN review_share_phone boolean NOT NULL DEFAULT false;
