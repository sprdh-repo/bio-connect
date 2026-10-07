-- Broadcast emails sent from the admin console. The audience is resolved once,
-- when staff confirm the send, into campaign_recipients, which is also the send
-- queue. A recipient appears at most once per campaign.
CREATE TABLE campaigns (
 id text PRIMARY KEY,
 template_id text NOT NULL,
 subject text NOT NULL,
 audience jsonb NOT NULL,
 created_by text NOT NULL REFERENCES staff,
 created_at timestamptz NOT NULL DEFAULT now(),
 cancelled_at timestamptz,
 cancelled_by text REFERENCES staff
);
CREATE INDEX campaigns_template ON campaigns(template_id);

CREATE TABLE campaign_recipients (
 id text PRIMARY KEY,
 campaign_id text NOT NULL REFERENCES campaigns,
 email text NOT NULL CHECK(email=lower(email)),
 name text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','sending','accepted','delivered','failed','uncertain','cancelled')),
 attempts integer NOT NULL DEFAULT 0,
 provider_id text,
 error_code text NOT NULL DEFAULT '',
 available_at timestamptz NOT NULL DEFAULT now(),
 claimed_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(campaign_id,email)
);
CREATE INDEX campaign_recipients_queue ON campaign_recipients(available_at) WHERE status='queued';
CREATE INDEX campaign_recipients_provider ON campaign_recipients(provider_id) WHERE provider_id IS NOT NULL;
CREATE INDEX campaign_recipients_email ON campaign_recipients(email);
