-- Contact exchange: an attendee chooses, from My passes in the app, whether
-- people who scan their badge also receive their email or phone. Off by default.
ALTER TABLE attendees
 ADD COLUMN share_email boolean NOT NULL DEFAULT false,
 ADD COLUMN share_phone boolean NOT NULL DEFAULT false,
 ADD COLUMN sharing_updated_at timestamptz;

-- Attendee feedback from the app: one answer per phone for the event and for
-- each session, replaced when the attendee changes it. device_id is a random
-- per-install value; nothing ties it to a registration.
CREATE TABLE feedback (
 id text PRIMARY KEY,
 device_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('event','session')),
 session_id text NOT NULL DEFAULT '',
 rating smallint NOT NULL CHECK(rating BETWEEN 1 AND 5),
 comment text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK((kind = 'event') = (session_id = '')),
 UNIQUE(device_id, kind, session_id)
);
CREATE INDEX feedback_target ON feedback(kind, session_id);

-- Feedback stays closed until staff open it. The new destinations join the
-- released menus: My agenda as a tab and shortcut, Contacts beside it, and
-- the feedback and after-event pages in the Guide, switched off for now.
UPDATE app_content SET updated_at = now(), document = document
 || '{"feedback":{"open":false,"intro":""}}'::jsonb
 || jsonb_build_object('menus', coalesce(document->'menus', '{}'::jsonb)
   || jsonb_build_object('tabs', coalesce(document#>'{menus,tabs}', '[]'::jsonb)
     || '[{"key":"agenda","title":"My agenda","subtitle":"","url":"","published":true}]'::jsonb)
   || jsonb_build_object('home_shortcuts', coalesce(document#>'{menus,home_shortcuts}', '[]'::jsonb)
     || '[{"key":"agenda","title":"My agenda","subtitle":"Your day plan","url":"","published":true},
          {"key":"contacts","title":"Contacts","subtitle":"Scan a badge","url":"","published":true}]'::jsonb)
   || jsonb_build_object('guide', '[{"key":"agenda","title":"My agenda","subtitle":"Saved sessions, speakers and exhibitors","url":"","published":true},
          {"key":"contacts","title":"Contacts","subtitle":"People you met, with your notes","url":"","published":true}]'::jsonb
     || coalesce(document#>'{menus,guide}', '[]'::jsonb)
     || '[{"key":"feedback","title":"Share feedback","subtitle":"Rate the event and the sessions you attended","url":"","published":false},
          {"key":"hub","title":"After Bio Connect","subtitle":"Your sessions, contacts and notes","url":"","published":false}]'::jsonb))
WHERE id = 'mobile';
