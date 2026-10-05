-- Staff edit the whole mobile app document from the console (/admin?view=mobile).
-- A revision guards against two windows overwriting each other.
ALTER TABLE app_content ADD COLUMN revision integer NOT NULL DEFAULT 1;

-- Every list entry gains its own "show in app" flag. Existing entries stay visible.
CREATE FUNCTION pg_temp.publish_all(items jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 SELECT coalesce(jsonb_agg(item || '{"published":true}' ORDER BY n), '[]'::jsonb)
 FROM jsonb_array_elements(coalesce(items, '[]'::jsonb)) WITH ORDINALITY AS t(item, n)
$$;

UPDATE app_content SET updated_at = now(), document = jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set(
  document - 'sponsor',
  '{themes}', pg_temp.publish_all(document->'themes')),
  '{sponsors}', pg_temp.publish_all(document->'sponsors')),
  '{ecosystem_partners}', pg_temp.publish_all(document->'ecosystem_partners')),
  '{leadership,people}', pg_temp.publish_all(document#>'{leadership,people}')),
  '{leadership,committee}', coalesce(document#>'{leadership,committee}', '{}'::jsonb)
    || jsonb_build_object('members', pg_temp.publish_all(document#>'{leadership,committee,members}')))
WHERE id = 'mobile';
DROP FUNCTION pg_temp.publish_all(jsonb);

-- The privacy link, the launch button and the app's menus were fixed in the app.
-- These defaults reproduce the released layout exactly.
UPDATE app_content SET document = document
 || '{"copy":{}}'::jsonb
 || jsonb_build_object('event', document->'event' || '{"privacy_url":"https://bioconnect.kerala.gov.in/privacy-policy"}')
 || jsonb_build_object('product_launch', document->'product_launch' || '{"apply_label":"Apply via Kerala Startup Mission"}')
 || $menus$
{"menus":{
 "tabs":[
  {"key":"sessions","title":"Sessions","subtitle":"","url":"","published":true},
  {"key":"speakers","title":"Speakers","subtitle":"","url":"","published":true}
 ],
 "home_shortcuts":[
  {"key":"sessions","title":"Sessions","subtitle":"Programme & timings","url":"","published":true},
  {"key":"venue","title":"Venue","subtitle":"Directions & arrival","url":"","published":true},
  {"key":"activities","title":"Activities","subtitle":"Discover & connect","url":"","published":true},
  {"key":"faqs","title":"FAQs","subtitle":"Event-day answers","url":"","published":true},
  {"key":"speakers","title":"Speakers","subtitle":"Meet the voices","url":"","published":true},
  {"key":"exhibitors","title":"Exhibitors","subtitle":"Explore the expo","url":"","published":true}
 ],
 "home_links":[
  {"key":"my_passes","title":"My passes","subtitle":"View your admission QR on this phone","url":"","published":true},
  {"key":"registration","title":"Registration & passes","subtitle":"Register or review delegate options","url":"","published":true},
  {"key":"explore","title":"Explore Bio Connect","subtitle":"Themes, ideas and programme highlights","url":"","published":true}
 ],
 "guide":[
  {"key":"venue","title":"Venue & directions","subtitle":"","url":"","published":true},
  {"key":"activities","title":"Activities","subtitle":"Discover what is happening","url":"","published":true},
  {"key":"faqs","title":"FAQs","subtitle":"Answers and event-day help","url":"","published":true},
  {"key":"exhibitors","title":"Exhibitors","subtitle":"Stalls and the expo line-up","url":"","published":true},
  {"key":"my_passes","title":"My passes","subtitle":"View your admission QR on this phone","url":"","published":true},
  {"key":"registration","title":"Registration & passes","subtitle":"Delegate and exhibition options","url":"","published":true},
  {"key":"brochure","title":"Event brochure","subtitle":"View the programme overview","url":"","published":true},
  {"key":"product_launch","title":"Product launch","subtitle":"Kerala Startup Mission showcase","url":"","published":true},
  {"key":"sponsors","title":"Sponsors","subtitle":"","url":"","published":true},
  {"key":"leadership","title":"Leadership","subtitle":"State leadership and the advisory committee","url":"","published":true},
  {"key":"privacy","title":"Privacy policy","subtitle":"How we use and protect your information","url":"","published":true}
 ]
}}
$menus$::jsonb
WHERE id = 'mobile';

-- Portraits become editable URLs instead of slugs into the website's asset folder.
ALTER TABLE speakers ADD COLUMN image_url text NOT NULL DEFAULT '';
UPDATE speakers SET image_url = 'https://bioconnect.kerala.gov.in/assets/speakers/' || image_slug || '.webp'
WHERE image_slug <> '';
ALTER TABLE speakers DROP COLUMN image_slug;
