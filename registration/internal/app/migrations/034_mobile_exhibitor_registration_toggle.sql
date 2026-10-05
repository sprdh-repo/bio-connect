-- Exhibition bookings have closed. Keep the directory of participating
-- exhibitors, but let staff independently restore the mobile booking section.
UPDATE app_content
SET updated_at = now(),
    document = jsonb_set(
      document,
      '{event,show_exhibitor_registration}',
      'false'::jsonb,
      true
    )
WHERE id = 'mobile';
