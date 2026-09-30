CREATE TABLE event_guide (
 id text PRIMARY KEY CHECK (id = 'mobile'),
 document jsonb NOT NULL CHECK (jsonb_typeof(document) = 'object'),
 revision integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO event_guide(id,document) VALUES ('mobile', '{"sessions":[],"activities":[],"faqs":[],"venue":{}}');
