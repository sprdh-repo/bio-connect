-- Staff can email registrations still awaiting payment a reminder that carries
-- a single-use link back to the registration.
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_purpose_check,
 ADD CONSTRAINT delivery_jobs_purpose_check CHECK(purpose IN ('pass','pack','recovery','registration','payment_reminder'));
