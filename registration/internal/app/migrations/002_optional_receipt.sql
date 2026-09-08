-- Payment evidence no longer requires a receipt upload. Reviewers verify the
-- bank reference and date against the SBI reconciliation report; a receipt file
-- is still accepted and linked when one is provided.
ALTER TABLE payment_submissions ALTER COLUMN receipt_id DROP NOT NULL;
