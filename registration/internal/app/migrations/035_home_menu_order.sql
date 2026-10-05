-- Five bottom tabs crowded the bar: My agenda moves off it (staff can still
-- add it back from the console). Home's shortcut tiles become Speakers,
-- Exhibitors, Venue, My agenda, Contacts and Moments, keeping any wording
-- staff gave an entry, and the home list ends with Sponsors.
UPDATE app_content SET updated_at = now(), document = jsonb_set(jsonb_set(jsonb_set(document,
  '{menus,tabs}',
  (SELECT coalesce(jsonb_agg(item ORDER BY n), '[]'::jsonb)
   FROM jsonb_array_elements(coalesce(document#>'{menus,tabs}', '[]'::jsonb)) WITH ORDINALITY AS t(item, n)
   WHERE item->>'key' <> 'agenda')),
  '{menus,home_shortcuts}',
  (SELECT jsonb_agg(coalesce(
     (SELECT item FROM jsonb_array_elements(coalesce(document#>'{menus,home_shortcuts}', '[]'::jsonb)) AS s(item)
      WHERE item->>'key' = d.item->>'key' LIMIT 1),
     d.item) ORDER BY d.n)
   FROM jsonb_array_elements('[
     {"key":"speakers","title":"Speakers","subtitle":"Meet the voices","url":"","published":true},
     {"key":"exhibitors","title":"Exhibitors","subtitle":"Explore the expo","url":"","published":true},
     {"key":"venue","title":"Venue","subtitle":"Directions & arrival","url":"","published":true},
     {"key":"agenda","title":"My agenda","subtitle":"Your day plan","url":"","published":true},
     {"key":"contacts","title":"Contacts","subtitle":"Scan a badge","url":"","published":true},
     {"key":"moments","title":"Moments","subtitle":"Find your event photos","url":"","published":true}
   ]'::jsonb) WITH ORDINALITY AS d(item, n))),
  '{menus,home_links}',
  coalesce(document#>'{menus,home_links}', '[]'::jsonb) || CASE
    WHEN EXISTS (SELECT 1 FROM jsonb_array_elements(coalesce(document#>'{menus,home_links}', '[]'::jsonb)) AS l(item)
                 WHERE item->>'key' = 'sponsors') THEN '[]'::jsonb
    ELSE '[{"key":"sponsors","title":"Sponsors","subtitle":"","url":"","published":true}]'::jsonb
  END)
WHERE id = 'mobile';
