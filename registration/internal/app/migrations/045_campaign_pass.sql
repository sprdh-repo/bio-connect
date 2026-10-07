-- Campaigns that attach each recipient's own pass record which pass, chosen
-- when the audience is snapshotted.
ALTER TABLE campaign_recipients ADD COLUMN pass_id text REFERENCES passes;
