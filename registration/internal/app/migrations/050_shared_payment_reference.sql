-- One bank or UPI payment can now cover several registrations: the spot desk
-- and the console record a group paying together under one reference. Reuse
-- is allowed and written to the audit log (noteReusedReference) instead of
-- refused; the plain index keeps that lookup fast.
DROP INDEX approved_transaction;
CREATE INDEX payment_verified_reference ON payment_submissions(verified_reference) WHERE verified_at IS NOT NULL;
