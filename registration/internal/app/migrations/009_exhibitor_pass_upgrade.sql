-- Exhibitor pass allowances rise from 3 / 2 / 2 to 5 / 3 / 2 (premium,
-- standard, table) at no extra fee. Unlike 003, which cut allowances and so
-- left existing registrations alone, this one grants the new allowance to
-- every live registration as well. Their submitted attendees keep their
-- places and passes; the extra places start empty and are filled later from
-- the management page or the staff console (see AddAttendee). Registrations
-- made before 003 with a larger roster keep it.
UPDATE categories SET roster_count=5 WHERE id='premium';
UPDATE categories SET roster_count=3 WHERE id='standard';

WITH upgraded AS (
 UPDATE registrations r SET roster_count=c.roster_count,updated_at=now()
 FROM categories c
 WHERE c.id=r.category_id AND c.id IN ('premium','standard')
  AND r.status NOT IN ('rejected','cancelled') AND r.roster_count<c.roster_count
 RETURNING r.id,r.roster_count
)
INSERT INTO audit_events(registration_id,action,detail)
SELECT id,'roster_upgraded','pass allowance raised to '||roster_count FROM upgraded;

-- The email that tells an upgraded exhibitor they have places to fill.
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_purpose_check,
 ADD CONSTRAINT delivery_jobs_purpose_check CHECK(purpose IN ('pass','pack','recovery','registration','payment_reminder','roster_notice'));
