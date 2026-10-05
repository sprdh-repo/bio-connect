-- Moments stays hidden until staff set the event's external album ID.
UPDATE app_content
SET document = jsonb_set(document, '{event,moments_album_id}', '""'::jsonb, true)
WHERE id = 'mobile' AND NOT (document->'event' ? 'moments_album_id');

-- Add one ready-to-use Guide entry. Released apps ignore the unknown key;
-- current apps hide it while the album ID is blank.
UPDATE app_content
SET document = jsonb_set(
  document,
  '{menus,guide}',
  coalesce(document#>'{menus,guide}', '[]'::jsonb) ||
    '[{"key":"moments","title":"Moments album","subtitle":"Find and save your event photos","url":"","published":true}]'::jsonb,
  true
)
WHERE id = 'mobile'
  AND NOT EXISTS (
    SELECT 1
    FROM jsonb_array_elements(coalesce(document#>'{menus,guide}', '[]'::jsonb)) item
    WHERE item->>'key' = 'moments'
  );
