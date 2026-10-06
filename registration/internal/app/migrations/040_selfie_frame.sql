-- The selfie frame joins the Guide after Moments (or at the end without it)
-- and leads the home links. App builds without the screen drop the unknown key.
UPDATE app_content SET updated_at = now(), document = jsonb_set(jsonb_set(document,
  '{menus,guide}',
  (SELECT jsonb_agg(item ORDER BY n, after)
   FROM (
     SELECT item, n, 0 AS after
     FROM jsonb_array_elements(coalesce(document#>'{menus,guide}', '[]'::jsonb)) WITH ORDINALITY AS g(item, n)
     UNION ALL
     SELECT '{"key":"selfie_frame","title":"Selfie frame","subtitle":"Share that you are at Bio Connect","url":"","published":true}'::jsonb,
       coalesce((SELECT n FROM jsonb_array_elements(coalesce(document#>'{menus,guide}', '[]'::jsonb)) WITH ORDINALITY AS g(item, n)
                 WHERE item->>'key' = 'moments' LIMIT 1), 2147483647),
       1
   ) entries)),
  '{menus,home_links}',
  '[{"key":"selfie_frame","title":"Selfie frame","subtitle":"Share that you are at Bio Connect","url":"","published":true}]'::jsonb
    || coalesce(document#>'{menus,home_links}', '[]'::jsonb))
WHERE id = 'mobile';
