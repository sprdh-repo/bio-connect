-- A registration can carry a coupon. The discount is frozen on the row when the
-- registration is saved, so a later change to the coupon table in code never
-- changes what an existing registrant owes. A coupon registration pays by
-- direct bank transfer instead of SBI Collect.
ALTER TABLE registrations ADD COLUMN coupon_code text NOT NULL DEFAULT '',
 ADD COLUMN discount_percent integer NOT NULL DEFAULT 0,
 ADD CONSTRAINT registration_discount_percent CHECK(discount_percent BETWEEN 0 AND 99),
 ADD CONSTRAINT registration_coupon_discount CHECK((coupon_code='')=(discount_percent=0));
